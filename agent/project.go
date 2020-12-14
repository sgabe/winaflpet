// +build windows

package main

import (
	"errors"

	"github.com/rs/xid"
)

type Project struct {
	Jobs []Job
}

func (p *Project) AddJob(j Job) []Job {
	p.Jobs = append(p.Jobs, j)
	return p.Jobs
}

func (p *Project) RemoveJob(i int) []Job {
	copy(p.Jobs[i:], p.Jobs[i+1:])

	if len(p.Jobs) == 1 {
		p.Jobs = nil
	} else {
		p.Jobs[i] = p.Jobs[len(p.Jobs)-1]
		p.Jobs = p.Jobs[:len(p.Jobs)-1]
	}

	return p.Jobs
}

func (p *Project) GetJob(guid string) (Job, int, error) {
	var j Job

	GUID, err := xid.FromString(guid)
	if err != nil {
		return j, 0, nil
	}

	for index, j := range p.Jobs {
		if j.GUID == GUID {
			return j, index, nil
		}
	}

	return j, 0, errors.New("Job not found")
}

func (p *Project) FindJob(GUID xid.ID) (bool, error) {
	for _, j := range p.Jobs {
		if j.GUID == GUID {
			return true, nil
		}
	}

	return false, nil
}
