//go:build windows
// +build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
	"golang.org/x/sys/windows"
)

const BUGID_BUG_NOT_DETECTED = "The application terminated without a bug being detected"

type Crash struct {
	JobGUID     xid.ID `json:"jguid"`
	FuzzerID    string `json:"fuzzerid"`
	BugID       string `json:"bugid"`
	Module      string `json:"mod"`
	Function    string `json:"func"`
	Description string `json:"desc"`
	Impact      string `json:"imp"`
	Args        string `json:"args"`
}

func newCrash(jobGUID xid.ID, fuzzerID string, function string, args string) Crash {
	c := new(Crash)
	c.JobGUID = jobGUID
	c.FuzzerID = fuzzerID
	c.Function = function
	c.Args = args
	return *c
}

func (c Crash) Verify() (Crash, error) {
	return c.verify("")
}

// A report directory enables HTML output; ordinary verification remains lightweight.
func (c Crash) verify(reportDir string) (Crash, error) {
	GUID := c.JobGUID.String()

	job, _, err := project.GetJob(GUID)
	if err != nil {
		logger.Error(err)
		return c, err
	}

	python, err := exec.LookPath(path.Join(job.PyDir, "python.exe"))
	if err != nil {
		logger.Error(err)
		return c, err
	}

	bugid, err := exec.LookPath(path.Join(job.BugIdDir, "BugId.cmd"))
	if err != nil {
		logger.Error(err)
		return c, err
	}

	jobHandle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return c, err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags =
		windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	windows.SetInformationJobObject(
		jobHandle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)

	targetCmd, targetArgs := splitCmdLine(job.TargetApp)

	dir := filepath.Dir(targetCmd)
	base := filepath.Base(targetCmd)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	phPath := filepath.Join(dir, name+".ph"+ext)
	if err := copyFile(targetCmd, phPath); err == nil {
		targetCmd = phPath
	} else if _, err := os.Stat(phPath); err == nil {
		targetCmd = phPath
	}

	reportOptions := " --bGenerateReportHTML=false"
	if reportDir != "" {
		reportOptions = " --bGenerateReportHTML=true --reports=" + windows.EscapeArg(reportDir)
	}
	args := fmt.Sprintf("-q"+
		" --collateral=1"+
		" --bShowLicenseAndDonationInfo=false"+
		"%s"+
		" --cBugId.bEnsurePageHeap=false"+
		" --isa=%s"+
		" %s --"+
		" %s"+
		" -f %s", // Sample delivery via file.
		reportOptions, job.TargetArch, targetCmd, targetArgs, c.Args)
	cmd := exec.Command(bugid, args)
	cmd.Dir = job.BugIdDir
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("PYTHON=%s", python),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.SysProcAttr.CmdLine = strings.Join(cmd.Args, ` `)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		logger.Error(err)
		windows.CloseHandle(jobHandle)
		return c, err
	}

	if reportDir == "" {
		go cmd.Wait()
	}

	procHandle, err := windows.OpenProcess(
		windows.PROCESS_ALL_ACCESS,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		windows.CloseHandle(jobHandle)
		if reportDir != "" {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return c, err
	}
	defer windows.CloseHandle(procHandle)

	err = windows.AssignProcessToJobObject(jobHandle, procHandle)
	if err != nil {
		windows.CloseHandle(jobHandle)
		if reportDir != "" {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return c, err
	}

	detected := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.Contains(s, BUGID_BUG_NOT_DETECTED) {
			if reportDir == "" {
				jobCleanup(jobHandle, 2*time.Second)
				return c, errors.New("no bug detected")
			}
		}
		if m := regexp.MustCompile(`Id @ Location: +(.*) @ (.*)`).FindStringSubmatch(s); len(m) > 0 {
			c.BugID = m[1]
			re := regexp.MustCompile(`[!+]`)
			parts := re.Split(m[2], -1)
			if len(parts) > 1 {
				c.Module = parts[1]
			}
			if len(parts) > 2 {
				c.Function = parts[2]
			}
		}
		if m := regexp.MustCompile(`Description: +(.*)`).FindStringSubmatch(s); len(m) > 0 {
			c.Description = m[1]
		}
		if m := regexp.MustCompile(`Security impact: +(.*)`).FindStringSubmatch(s); len(m) > 0 {
			c.Impact = m[1]
			detected = true
			if reportDir == "" {
				jobCleanup(jobHandle, 2*time.Second)
				return c, nil
			}
		}
	}

	if reportDir != "" {
		if err := scanner.Err(); err != nil {
			jobCleanup(jobHandle, 2*time.Second)
			_ = cmd.Wait()
			return c, err
		}
		// Drain stdout before waiting: the HTML is saved after the bug summary.
		_ = cmd.Wait()
	}
	jobCleanup(jobHandle, 2*time.Second)
	if detected {
		return c, nil
	}
	return c, errors.New("no bug detected")
}

func verifyCrash(c *gin.Context) {
	var crash Crash
	if err := c.ShouldBindJSON(&crash); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	crash, err := crash.Verify()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, crash)
}

func downloadCrash(c *gin.Context) {
	var crash Crash
	if err := c.ShouldBindJSON(&crash); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	filePath := crash.Args
	if !fileExists(filePath) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "The provided file path is invalid.",
		})
		return
	}

	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	c.Header("Content-Type", "application/octet-stream")

	c.File(filePath)
}

func reportCrash(c *gin.Context) {
	var crash Crash
	if err := c.ShouldBindJSON(&crash); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	directory, err := os.MkdirTemp("", "winaflpet-report-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.RemoveAll(directory)
	if _, err = crash.verify(directory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.FileAttachment(filepath.Join(directory, entry.Name()), filepath.Base(crash.Args)+".html")
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "BugId did not generate an HTML report."})
}
