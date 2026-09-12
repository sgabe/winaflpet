package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/rs/xid"
)

type Alert struct {
	Jobs []Job
}

func (a *Alert) AddJob(j Job) []Job {
	a.Jobs = append(a.Jobs, j)
	return a.Jobs
}

func (a *Alert) RemoveJob(i int) []Job {
	copy(a.Jobs[i:], a.Jobs[i+1:])

	if len(a.Jobs) == 1 {
		a.Jobs = nil
	} else {
		a.Jobs[i] = a.Jobs[len(a.Jobs)-1]
		a.Jobs = a.Jobs[:len(a.Jobs)-1]
	}

	return a.Jobs
}

func (a *Alert) GetJob(GUID xid.ID) (Job, int, error) {
	var j Job

	for index, j := range a.Jobs {
		if j.GUID == GUID {
			return j, index, nil
		}
	}

	return j, 0, errors.New("Job not found")
}

func (a *Alert) FindJob(GUID xid.ID) (bool, error) {
	for _, j := range a.Jobs {
		if j.GUID == GUID {
			return true, nil
		}
	}

	return false, nil
}

func (a *Alert) Monitor(j Job, m *mail.Address) {
	settings, err := loadSettings()
	if err != nil {
		log.Println("Unable to load alert settings:", err)
		if _, i, err := a.GetJob(j.GUID); err == nil {
			a.RemoveJob(i)
		}
		return
	}
	ticker := time.NewTicker(time.Duration(settings.Interval) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		updated, err := loadSettings()
		if err != nil {
			log.Println("Unable to refresh alert settings:", err)
		} else {
			settings = updated
		}
		ticker.Reset(time.Duration(settings.Interval) * time.Minute)
		j.Recorder.Load()
		if j.Status == 0 {
			ticker.Stop()
			if _, i, err := a.GetJob(j.GUID); err == nil {
				a.RemoveJob(i)
			}
			return
		}

		request := gorequest.New()
		request.Debug = false

		agent, _ := j.GetAgent()
		targetURL := fmt.Sprintf("http://%s:%d/job/%s/collect", agent.Host, agent.Port, j.GUID)

		var crashesTemp []Crash
		resp, _, errs := request.Post(targetURL).Set("X-Auth-Key", agent.Key).EndStruct(&crashesTemp)
		if errs != nil || resp.StatusCode != http.StatusOK {
			ticker.Stop()
			if _, i, err := a.GetJob(j.GUID); err == nil {
				a.RemoveJob(i)
			}
			return
		}

		resumedJob := false
		if j.Input == "-" {
			resumedJob = true
		}

		var crashes []Crash
		for _, crash := range crashesTemp {
			c := newCrash()
			c.JobID = j.ID
			c.FuzzerID = crash.FuzzerID

			recentCrash := false
			for _, i := range crashesTemp {
				if i.FuzzerID == c.FuzzerID && strings.Contains(i.Args, "\\crashes\\") {
					recentCrash = true
					break
				}
			}

			re := regexp.MustCompile(c.FuzzerID + `\\crashes_\d{14}\\`)
			backedUpCrash := re.MatchString(crash.Args)

			// Avoid duplicate crash records when resuming aborted jobs.
			if resumedJob && !recentCrash && backedUpCrash {
				c.Args = re.ReplaceAllString(crash.Args, c.FuzzerID+"\\crashes\\")
				if err := c.LoadByJobIDArgs(); err == nil {
					c.Args = crash.Args
					if err := c.Update(); err != nil {
						log.Println(err)
					}
					continue
				}
			}

			c.Args = crash.Args
			if err := c.LoadByJobIDArgs(); err != nil {
				if err := c.Insert(); err != nil {
					log.Println(err)
					break
				}
				crashes = append(crashes, *c)
			}
		}

		if len(crashes) > 0 {
			host := settings.Host
			port := settings.Port
			username := settings.Username
			password := settings.Password

			addr := net.JoinHostPort(host, strconv.Itoa(port))
			auth := smtp.PlainAuth("", username, password, host)
			to := []string{m.Address}

			message := []byte(fmt.Sprintf("From: %s\r\n", username) +
				fmt.Sprintf("To: %s\r\n", m.Address) +
				fmt.Sprintf("Subject: WinAFL Pet found %d new crashes for job %s\r\n", len(crashes), j.Name) +
				"\r\n" +
				"WinAFL Pet has found the following crashes since the last check:\r\n\r\n")

			for _, crash := range crashes {
				filePath := strings.Split(crash.Args, "\\")
				fileName := filePath[len(filePath)-1]
				message = append(message, fmt.Sprintf("%s\r\n", fileName)...)
			}

			err := smtp.SendMail(addr, auth, username, to, message)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
