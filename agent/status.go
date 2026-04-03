package main

import (
	"sync"
	"time"
)

type status string

const (
	starting      status = "starting"
	bootstrapping status = "bootstrapping"
	running       status = "running"
	failed        status = "failed"
)

var states = struct {
	sync.RWMutex
	m map[string]status
}{
	m: make(map[string]status),
}

var pids = struct {
	sync.RWMutex
	m map[string]int
}{
	m: make(map[string]int),
}

func setPID(key string, pid int) {
	pids.Lock()
	defer pids.Unlock()
	pids.m[key] = pid
}

func getPID(key string) int {
	pids.RLock()
	defer pids.Unlock()
	return pids.m[key]
}

func setStatus(key string, s status) {
	states.Lock()
	states.m[key] = s
	states.Unlock()
}

func getStatus(key string) status {
	states.Lock()
	defer states.Unlock()
	return states.m[key]
}

func waitUntilStarted(key string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status := getStatus(key)

		if status == running {
			return true
		}

		if status == failed {
			return false
		}

		time.Sleep(200 * time.Millisecond)
	}

	return false
}
