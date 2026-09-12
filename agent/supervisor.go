//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"
)

type InstanceState struct {
	FID    int    `json:"fid"`
	RunID  string `json:"run_id"`
	Status status `json:"status"`
	PID    int    `json:"pid,omitempty"`
	Error  string `json:"error,omitempty"`
}

type JobSnapshot struct {
	Instances []InstanceState `json:"instances"`
	Msg       string          `json:"msg"`
	Known     bool            `json:"known"`
	Protocol  int             `json:"protocol"`
}

type instance struct {
	state   InstanceState
	proc    managedProcess
	done    chan struct{}
	failure string
}

type run struct {
	job       Job
	fid       int
	hash      [32]byte
	ctx       context.Context
	cancel    context.CancelFunc
	instances []*instance
	completed bool
}

type jobRun struct {
	config    [32]byte
	instances map[int]*instance
	requests  map[string]*run
	cancelled map[string]bool
	completed []string
}

type manager struct {
	mu             sync.Mutex
	jobs           map[xid.ID]*jobRun
	launch         func(*exec.Cmd) (managedProcess, error)
	command        func(Job, int) (*exec.Cmd, error)
	startupTimeout time.Duration
	shuttingDown   bool
}

func newManager() *manager {
	return &manager{jobs: make(map[xid.ID]*jobRun), launch: launchProcess, command: func(j Job, id int) (*exec.Cmd, error) { return j.GetCmd(id) }, startupTimeout: 10 * time.Minute}
}

var supervisor = newManager()

const maxCompletedRequests = 64

func configHash(j Job) [32]byte {
	j.Status = 0
	b, _ := json.Marshal(j)
	return sha256.Sum256(b)
}

func runCompleted(r *run) bool {
	if len(r.instances) == 0 {
		return false
	}
	for _, i := range r.instances {
		if active(i.state.Status) {
			return false
		}
	}
	return true
}

// Keep a bounded completed-request history so recent retries remain
// idempotent without retaining every run for the lifetime of the Agent.
func pruneCompletedLocked(job *jobRun) {
	if job == nil {
		return
	}
	for requestID, r := range job.requests {
		if runCompleted(r) && !r.completed {
			r.completed = true
			job.completed = append(job.completed, requestID)
		}
	}
	for len(job.completed) > maxCompletedRequests {
		requestID := job.completed[0]
		job.completed = job.completed[1:]
		delete(job.requests, requestID)
	}
}

func (m *manager) start(j Job, fid int, requestID string) (JobSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown {
		return JobSnapshot{}, errors.New("Agent is shutting down")
	}

	hash := configHash(j)
	job := m.jobs[j.GUID]
	if job == nil {
		job = &jobRun{instances: make(map[int]*instance), requests: make(map[string]*run), cancelled: make(map[string]bool)}
		m.jobs[j.GUID] = job
	}
	pruneCompletedLocked(job)

	if job.cancelled[requestID] {
		return JobSnapshot{}, errors.New("Start request was cancelled")
	}

	if previous := job.requests[requestID]; previous != nil {
		if previous.hash != hash || previous.fid != fid {
			return JobSnapshot{}, errors.New("Request ID already used with different parameters")
		}
		return m.snapshotLocked(j.GUID), nil
	}

	for id, i := range job.instances {
		if active(i.state.Status) && (job.config != hash || fid == 0 || fid == id) {
			return JobSnapshot{}, fmt.Errorf("Instance %d is already active", id)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &run{job: j, fid: fid, hash: hash, ctx: ctx, cancel: cancel}
	for id := 1; id <= j.Cores; id++ {
		if fid != 0 && fid != id {
			continue
		}
		i := &instance{state: InstanceState{FID: id, RunID: requestID, Status: queued}, done: make(chan struct{})}
		job.instances[id] = i
		r.instances = append(r.instances, i)
	}

	for id, i := range job.instances {
		if id > j.Cores && !active(i.state.Status) {
			delete(job.instances, id)
		}
	}

	job.config = hash
	job.requests[requestID] = r
	project.AddJob(j)
	snapshot := m.snapshotLocked(j.GUID)
	go m.execute(r)
	return snapshot, nil
}

func (m *manager) snapshot(guid xid.ID) JobSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(guid)
}

func (m *manager) snapshotLocked(guid xid.ID) JobSnapshot {
	s := JobSnapshot{Instances: []InstanceState{}, Protocol: 2}
	job := m.jobs[guid]
	if job == nil {
		s.Msg = "No managed run in this agent session"
		return s
	}

	s.Known = true
	pruneCompletedLocked(job)
	j, _, _ := project.GetJob(guid.String())
	counts := make(map[status]int)
	for id := 1; id <= j.Cores; id++ {
		state := InstanceState{FID: id, Status: stopped}
		if i := job.instances[id]; i != nil {
			state = i.state
		}
		s.Instances = append(s.Instances, state)
		counts[state.Status]++
	}

	s.Msg = fmt.Sprintf("%d running, %d queued/starting, %d stopping, %d stopped, %d failed", counts[running], counts[queued]+counts[starting]+counts[bootstrapping], counts[stopping], counts[stopped], counts[failed]+counts[stopFailed])
	return s
}

func (m *manager) execute(r *run) {
	for _, i := range r.instances {
		m.mu.Lock()
		if r.ctx.Err() != nil {
			if i.state.Status == queued {
				i.state.Status = stopped
				close(i.done)
				pruneCompletedLocked(m.jobs[r.job.GUID])
			}
			m.mu.Unlock()
			continue
		}

		i.state.Status = starting
		m.mu.Unlock()
		cmd, err := m.command(r.job, i.state.FID)
		m.mu.Lock()
		if r.ctx.Err() != nil {
			i.state.Status = stopped
			close(i.done)
			pruneCompletedLocked(m.jobs[r.job.GUID])
			m.mu.Unlock()
			continue
		}

		output := newStartupOutput()
		if err == nil {
			cmd.Stdout = output
			cmd.Stderr = output
			cmd.WaitDelay = 3 * time.Second
			i.proc, err = m.launch(cmd)
		}

		if err != nil {
			i.state.Status = failed
			i.state.Error = err.Error()
			close(i.done)
			pruneCompletedLocked(m.jobs[r.job.GUID])
			m.mu.Unlock()
			continue
		}

		i.state.PID = i.proc.PID()
		i.state.Status = bootstrapping
		m.mu.Unlock()
		go m.wait(r, i)
		timer := time.NewTimer(m.startupTimeout)
		select {
		case err = <-output.ready:
			if err == nil {
				m.mu.Lock()
				if i.state.Status == bootstrapping {
					i.state.Status = running
				}
				m.mu.Unlock()
			} else {
				m.fail(i, err)
			}
		case <-r.ctx.Done():
		case <-i.done:
		case <-timer.C:
			m.fail(i, errors.New("Startup deadline exceeded"))
		}

		timer.Stop()
	}
}

func (m *manager) fail(i *instance, cause error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i.state.Status == stopped || i.state.Status == failed || i.state.Status == stopping || i.state.Status == stopFailed {
		return
	}

	i.failure = cause.Error()
	i.state.Error = i.failure
	i.state.Status = stopping
	if err := i.proc.Stop(); err != nil {
		i.state.Status = stopFailed
		i.state.Error = fmt.Sprintf("%s; terminate: %v", i.failure, err)
	}
}

func (m *manager) wait(r *run, i *instance) {
	err := i.proc.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	i.state.PID = 0
	var cleanup *cleanupError

	if errors.As(err, &cleanup) {
		i.state.Status = stopFailed
		i.state.Error = err.Error()
		close(i.done)
		pruneCompletedLocked(m.jobs[r.job.GUID])
		return
	}

	if r.ctx.Err() != nil && i.failure == "" {
		i.state.Status = stopped
		i.state.Error = ""
		close(i.done)
		pruneCompletedLocked(m.jobs[r.job.GUID])
		return
	}

	i.state.Status = failed
	if i.failure != "" {
		i.state.Error = i.failure
	} else if err != nil {
		i.state.Error = err.Error()
	} else {
		i.state.Error = "Fuzzer exited"
	}

	close(i.done)
	pruneCompletedLocked(m.jobs[r.job.GUID])
}

type startupOutput struct {
	mu       sync.Mutex
	tail     string
	ready    chan error
	reported bool
}

func newStartupOutput() *startupOutput {
	return &startupOutput{ready: make(chan error, 1)}
}

func (o *startupOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.tail += string(p)
	if len(o.tail) > 65536 {
		o.tail = o.tail[len(o.tail)-65536:]
	}

	if !o.reported {
		clean := stripAnsi(o.tail)
		if strings.Contains(clean, AFL_SUCCESS_MSG) {
			o.ready <- nil
			o.reported = true
		} else if strings.Contains(clean, "PROGRAM ABORT") || strings.Contains(clean, "OS message") {
			o.ready <- fmt.Errorf("Startup failed: %s", clean)
			o.reported = true
		}
	}

	return len(p), nil
}

// Cancellation and launch registration share mu. Once this returns, no queued
// launch from an existing run can cross the process creation boundary.
func (m *manager) stop(guid xid.ID) (JobSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[guid]
	if job == nil {
		return JobSnapshot{}, errors.New("No managed run in this agent session")
	}
	return m.stopLocked(guid, job), nil
}

func (m *manager) stopLocked(guid xid.ID, job *jobRun) JobSnapshot {
	pruneCompletedLocked(job)
	for _, r := range job.requests {
		r.cancel()
	}

	for _, i := range job.instances {
		if !active(i.state.Status) {
			continue
		}

		if i.proc == nil {
			if i.state.Status == queued {
				i.state.Status = stopped
				close(i.done)
			} else {
				i.state.Status = stopping
			}
			continue
		}

		i.state.Status = stopping
		if err := i.proc.Stop(); err != nil {
			i.state.Status = stopFailed
			i.state.Error = fmt.Sprintf("Stop failed: %v", err)
		}
	}
	pruneCompletedLocked(job)

	return m.snapshotLocked(guid)
}

func (m *manager) shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shuttingDown = true
	guids := make([]xid.ID, 0, len(m.jobs))
	for guid := range m.jobs {
		guids = append(guids, guid)
	}

	m.mu.Unlock()
	for _, guid := range guids {
		if _, err := m.stop(guid); err != nil {
			return err
		}
	}

	m.mu.Lock()
	var instances []*instance
	for _, job := range m.jobs {
		for _, i := range job.instances {
			instances = append(instances, i)
		}
	}

	m.mu.Unlock()
	for _, i := range instances {
		select {
		case <-i.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, i := range instances {
		if active(i.state.Status) {
			return fmt.Errorf("Instance %d: %s", i.state.FID, i.state.Error)
		}
	}

	return nil
}

func (m *manager) stopRequests(j Job, ids []string) (JobSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[j.GUID]

	if job == nil {
		if len(ids) == 0 {
			return JobSnapshot{}, errors.New("No managed run in this agent session")
		}
		job = &jobRun{config: configHash(j), instances: make(map[int]*instance), requests: make(map[string]*run), cancelled: make(map[string]bool)}
		m.jobs[j.GUID] = job
		project.AddJob(j)
	}

	for _, id := range ids {
		job.cancelled[id] = true
	}

	return m.stopLocked(j.GUID, job), nil
}
