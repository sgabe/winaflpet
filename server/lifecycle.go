package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type InstanceState struct {
	FID    int    `json:"fid"`
	RunID  string `json:"run_id,omitempty"`
	Status string `json:"status"`
	PID    int    `json:"pid,omitempty"`
	Error  string `json:"error,omitempty"`
}

type JobSnapshot struct {
	Known      bool            `json:"known"`
	Protocol   int             `json:"protocol"`
	Instances  []InstanceState `json:"instances"`
	Msg        string          `json:"msg"`
	Error      string          `json:"error,omitempty"`
	ObservedAt int64           `json:"observed_at"`
}

func validateSnapshot(s JobSnapshot, cores int) error {
	if cores < 1 || cores > 40 {
		return fmt.Errorf("Invalid instance count")
	}
	if s.Protocol != 2 || !s.Known || s.Error != "" {
		return fmt.Errorf("Agent did not return a known protocol 2 run: %s", s.Error)
	}
	if len(s.Instances) != cores {
		return fmt.Errorf("Incomplete agent snapshot")
	}
	seen := make(map[int]bool)
	for _, i := range s.Instances {
		if i.FID < 1 || i.FID > cores || seen[i.FID] {
			return fmt.Errorf("Invalid instance ID")
		}
		seen[i.FID] = true
		switch i.Status {
		case "queued", "starting", "bootstrapping", "running", "stopping", "stopped", "failed", "stop_failed":
		default:
			return fmt.Errorf("Unknown instance state %q", i.Status)
		}
	}
	return nil
}

// After an idle configuration is resized, the agent still describes the last
// run. Preserve its terminal results while showing newly configured slots idle.
func resizeIdleSnapshot(s JobSnapshot, cores int) (JobSnapshot, error) {
	if err := validateSnapshot(s, len(s.Instances)); err != nil {
		return s, err
	}
	if len(s.Instances) == cores {
		return s, nil
	}
	if cores < 1 || cores > 40 {
		return s, fmt.Errorf("Invalid instance count")
	}
	for _, i := range s.Instances {
		if activeState(i.Status) {
			return s, fmt.Errorf("Agent instance count differs while a run is active")
		}
	}
	previous := make(map[int]InstanceState)
	for _, i := range s.Instances {
		previous[i.FID] = i
	}
	s.Instances = nil
	for id := 1; id <= cores; id++ {
		i, ok := previous[id]
		if !ok {
			i = InstanceState{FID: id, Status: "stopped"}
		}
		s.Instances = append(s.Instances, i)
	}
	return s, nil
}

func snapshotStopped(s JobSnapshot) bool {
	if validateSnapshot(s, len(s.Instances)) != nil {
		return false
	}
	for _, i := range s.Instances {
		if i.Status != "stopped" || i.PID != 0 {
			return false
		}
	}
	return true
}

func activeState(s string) bool {
	return s == "queued" || s == "starting" || s == "bootstrapping" || s == "running" || s == "stopping" || s == "stop_failed"
}

var jobLocks sync.Map

func lockJob(guid string) func() {
	v, _ := jobLocks.LoadOrStore(guid, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func (j *Job) snapshot() (JobSnapshot, error) {
	var raw string
	err := db.QueryRow("SELECT snapshot FROM job_runtime WHERE guid = ?", j.GUID.String()).Scan(&raw)
	if err != nil {
		return JobSnapshot{}, err
	}
	var s JobSnapshot
	err = json.Unmarshal([]byte(raw), &s)
	return s, err
}

func (j *Job) saveSnapshot(s JobSnapshot) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT OR REPLACE INTO job_runtime(guid, snapshot) VALUES (?, ?)", j.GUID.String(), string(raw)); err != nil {
		return err
	}
	var bits status
	for _, i := range s.Instances {
		if i.Status == "running" {
			bits = setStatus(bits, statusMap[i.FID])
		}
	}
	if _, err = tx.Exec("UPDATE jobs SET status = ? WHERE guid = ?", bits, j.GUID.String()); err != nil {
		return err
	}
	for _, i := range s.Instances {
		if i.RunID != "" {
			if _, err = tx.Exec("DELETE FROM job_start_requests WHERE guid=? AND request_id=?", j.GUID.String(), i.RunID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (j *Job) queuedSnapshot(fid int) JobSnapshot {
	s, _ := j.snapshot()
	states := make(map[int]InstanceState)
	for _, i := range s.Instances {
		states[i.FID] = i
	}
	s.Instances = nil
	for id := 1; id <= j.Cores; id++ {
		i, ok := states[id]
		if !ok {
			i = InstanceState{FID: id, Status: "stopped"}
		}
		if fid == 0 || fid == id {
			i = InstanceState{FID: id, Status: "queued"}
		}
		s.Instances = append(s.Instances, i)
	}
	s.Msg = "Start accepted; waiting for agent progress"
	s.ObservedAt = time.Now().Unix()
	return s
}

func agentRequest(j *Job, action string, body interface{}, out interface{}) error {
	a, err := j.GetAgent()
	if err != nil {
		return err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s:%d/job/%s/%s", a.Host, a.Port, j.GUID, action), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-Auth-Key", a.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &failure)
		return &agentHTTPError{Code: resp.StatusCode, Message: failure.Error}
	}
	return json.Unmarshal(data, out)
}

// Call after a reachable protocol 2 agent reports no managed run. A confirmed
// stop permits a new run; forgetting active history or pending starts does not.
// This check is independent of Input and never erases the previous run.
func (j *Job) allowUnknownRun() error {
	pending, err := j.pendingStartIDs()
	if err != nil {
		return err
	}
	if len(pending) != 0 {
		return errors.New("Agent has no record of this run and a start request is still unconfirmed")
	}
	if j.Status != 0 {
		return errors.New("Agent has no record of this run; the previous run is not confirmed stopped")
	}
	previous, err := j.snapshot()
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// Validate every old slot, even if the configured core count was reduced.
	if !snapshotStopped(previous) {
		return errors.New("Agent has no record of this run; the previous run is not confirmed stopped")
	}
	return nil
}

// A configuration with no outstanding start or active recorded instance can be
// managed even when its agent is offline. Starting a run records the pending
// request before delivery, so uncertain starts still block configuration changes.
func (j *Job) CanChangeConfig() bool {
	pending, err := j.pendingStartIDs()
	if err != nil || len(pending) != 0 {
		return false
	}
	s, err := j.snapshot()
	if errors.Is(err, sql.ErrNoRows) {
		return j.Status == 0
	}
	if err != nil || validateSnapshot(s, len(s.Instances)) != nil {
		return false
	}
	for _, i := range s.Instances {
		if activeState(i.Status) || i.PID != 0 {
			return false
		}
	}
	return true
}

func (j *Job) deleteConfiguration() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM jobs WHERE id = ? AND guid = ?", j.ID, j.GUID.String()); err != nil {
		return err
	}
	for _, table := range []string{"job_runtime", "job_start_requests"} {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE guid = ?", j.GUID.String()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (j *Job) allowConfigChange() error {
	pending, err := j.pendingStartIDs()
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return errors.New("Resolve or cancel the unconfirmed start before editing or deleting this job")
	}
	if j.CanChangeConfig() {
		return nil
	}
	var s JobSnapshot
	if err := agentRequest(j, "check", nil, &s); err != nil {
		return err
	}
	// Configuration may have been edited on the server previously, so inspect
	// the agent's complete snapshot instead of assuming the new core count.
	if err := validateSnapshot(s, len(s.Instances)); err != nil {
		return err
	}
	for _, i := range s.Instances {
		if activeState(i.Status) || i.PID != 0 {
			return errors.New("Stop all instances before editing or deleting this job")
		}
	}
	return nil
}
