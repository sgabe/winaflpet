//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type managedProcess interface {
	PID() int
	Wait() error
	Stop() error
}

type cleanupError struct{ err error }

func (e *cleanupError) Error() string {
	return "Process-tree termination unconfirmed: " + e.err.Error()
}

func (e *cleanupError) Unwrap() error { return e.err }

type commandProcess struct {
	cmd      *exec.Cmd
	mu       sync.Mutex
	job      windows.Handle
	root     windows.Handle
	closeErr error
}

func launchProcess(cmd *exec.Cmd) (managedProcess, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if err = cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}

	fail := func(cause error) (managedProcess, error) {
		windows.CloseHandle(job)
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		if killErr != nil {
			return nil, fmt.Errorf("%w (launch cleanup: %v; wait: %v)", cause, killErr, waitErr)
		}
		return nil, cause
	}

	root, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(err)
	}

	if err = windows.AssignProcessToJobObject(job, root); err != nil {
		windows.CloseHandle(root)
		return fail(fmt.Errorf("assign fuzzer to Job Object: %w", err))
	}

	if err = resumeInitialThread(uint32(cmd.Process.Pid)); err != nil {
		windows.CloseHandle(root)
		return fail(err)
	}

	return &commandProcess{cmd: cmd, job: job, root: root}, nil
}

func resumeInitialThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}

	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}

		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}

		count, err := windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if err != nil {
			return err
		}

		if count != 1 {
			return fmt.Errorf("unexpected initial thread suspend count %d", count)
		}

		return nil
	}

	return fmt.Errorf("initial thread for process %d not found: %w", pid, err)
}

func (p *commandProcess) PID() int {
	return p.cmd.Process.Pid
}

func (p *commandProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return p.closeErr
	}
	return windows.TerminateJobObject(p.job, 1)
}

// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION from winnt.h.
type jobAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func (p *commandProcess) waitEmpty(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var info jobAccounting
		err := windows.QueryInformationJobObject(p.job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil)
		if err != nil {
			return err
		}

		if info.ActiveProcesses == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("%d processes remain in Job Object", info.ActiveProcesses)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func (p *commandProcess) Wait() error {
	result, err := windows.WaitForSingleObject(p.root, windows.INFINITE)
	if err == nil && result != windows.WAIT_OBJECT_0 {
		err = fmt.Errorf("unexpected process wait result %d", result)
	}

	stopErr := p.Stop()
	emptyErr := p.waitEmpty(10 * time.Second)
	if err == nil {
		err = errors.Join(stopErr, emptyErr)
	}

	waitErr := p.cmd.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	windows.CloseHandle(p.root)
	windows.CloseHandle(p.job)
	p.root = 0
	p.job = 0
	if err != nil {
		p.closeErr = &cleanupError{err}
		return p.closeErr
	}

	return waitErr
}
