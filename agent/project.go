//go:build windows

package main

import (
	"errors"
	"github.com/rs/xid"
	"sync"
)

type Project struct {
	mu   sync.RWMutex
	jobs map[xid.ID]Job
}

func (p *Project) AddJob(j Job) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.jobs == nil {
		p.jobs = make(map[xid.ID]Job)
	}
	p.jobs[j.GUID] = j
}
func (p *Project) GetJob(guid string) (Job, int, error) {
	id, err := xid.FromString(guid)
	if err != nil {
		return Job{}, 0, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	j, ok := p.jobs[id]
	if !ok {
		return Job{}, 0, errors.New("Job not found")
	}
	return j, 0, nil
}
