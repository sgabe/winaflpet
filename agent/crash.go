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

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
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

func newCrash(jobGUID xid.ID, fuzzerID string, args string) Crash {
	c := new(Crash)
	c.JobGUID = jobGUID
	c.FuzzerID = fuzzerID
	c.Args = args
	return *c
}

func (c Crash) Verify() (Crash, error) {
	GUID := c.JobGUID.String()

	job, _, err := project.GetJob(GUID)
	if err != nil {
		logger.Error(err)
		return c, err
	}

	bugid, err := exec.LookPath(path.Join(job.BugIdDir, "BugId.cmd"))
	if err != nil {
		logger.Error(err)
		return c, err
	}

	args := fmt.Sprintf("-q"+
		" --bShowLicenseAndDonationInfo=false"+
		" --bGenerateReportHTML=false"+
		" --cBugId.bEnsurePageHeap=false"+
		" --isa=%s %s -- %s", job.TargetArch, job.TargetApp, c.Args)
	cmd := exec.Command(bugid, args)
	cmd.Dir = job.BugIdDir
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("PYTHON=%s", path.Join(job.PyDir, "python.exe")),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.CmdLine = strings.Join(cmd.Args, ` `)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		logger.Error(err)
		return c, err
	}

	if cmd.Process != nil {
		buf := bufio.NewReader(stdout)
		for {
			line, _, _ := buf.ReadLine()
			s := string(line)
			if strings.Contains(s, BUGID_BUG_NOT_DETECTED) {
				return c, errors.New("no bug detected")
			}
			if m := regexp.MustCompile(`Id @ Location: +(.*) @ (.*)`).FindStringSubmatch(s); len(m) > 0 {
				c.BugID = m[1]
				re := regexp.MustCompile(`[!+]`)
				c.Module = re.Split(m[2], -1)[1]
				c.Function = re.Split(m[2], -1)[2]
			}
			if m := regexp.MustCompile(`Description: +(.*)`).FindStringSubmatch(s); len(m) > 0 {
				c.Description = m[1]
			}
			if m := regexp.MustCompile(`Security impact: +(.*)`).FindStringSubmatch(s); len(m) > 0 {
				c.Impact = m[1]
				return c, nil
			}
		}
	}

	return c, nil
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
