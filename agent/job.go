// +build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-ps"
	"github.com/rs/xid"
)

const (
	AFL_EXECUTABLE  = "afl-fuzz.exe"
	AFL_SUCCESS_MSG = "All set and ready to roll!"
	AFL_FAIL_REGEX  = `PROGRAM ABORT : (.*)`
	AFL_STATS_FILE  = "fuzzer_stats"
	AFL_PLOT_FILE   = "plot_data"
)

type Job struct {
	GUID           xid.ID `json:"guid"`
	Name           string `json:"name"`
	Description    string `json:"desc"`
	Banner         string `json:"banner"`
	Cores          int    `json:"cores"`
	Input          string `json:"input"`
	Output         string `json:"output"`
	Timeout        int    `json:"timeout"`
	InstMode       string `json:"inst_mode"`
	CoverageType   string `json:"cov_type"`
	CoverageModule string `json:"cov_module"`
	FuzzIter       int    `json:"fuzz_iter"`
	TargetModule   string `json:"target_module"`
	TargetMethod   string `json:"target_method"`
	TargetOffset   string `json:"target_offset"`
	TargetNArgs    int    `json:"target_nargs"`
	TargetApp      string `json:"target_app"`
	TargetArch     string `json:"target_arch"`
	AFLDir         string `json:"afl_dir"`
	DrioDir        string `json:"drio_dir"`
	PyDir          string `json:"py_dir"`
	BugIdDir       string `json:"bugid_dir"`
	Status         int    `json:"status"`
}

func newJob(GUID string) Job {
	j := new(Job)
	j.Cores = 1
	j.GUID, _ = xid.FromString(GUID)
	return *j
}

func (j Job) Start(fID string) error {
	afl, err := exec.LookPath(path.Join(j.AFLDir, AFL_EXECUTABLE))
	if err != nil {
		logger.Error(err)
		return err
	}

	app, err := exec.LookPath(j.TargetApp)
	if err != nil {
		logger.Error(err)
		return err
	}

	distMode := "-M"
	if j.Cores > 1 && fID != "fuzzer1" {
		distMode = "-S"
	}

	args := fmt.Sprintf("%s %s -i %s -o %s -D %s -t %d -- -covtype %s -coverage_module %s -fuzz_iterations %d -target_module %s -target_method %s -nargs %d -- %s @@",
		distMode,
		fID,
		j.Input,
		j.Output,
		j.DrioDir,
		j.Timeout,
		j.CoverageType,
		strings.Join(strings.Split(j.CoverageModule, ","), " -coverage_module "),
		j.FuzzIter,
		j.TargetModule,
		j.TargetMethod,
		j.TargetNArgs,
		app)

	cmd := exec.Command(afl, args)
	cmd.Dir = j.AFLDir
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.CmdLine = strings.Join(cmd.Args, ` `)
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		logger.Error(err)
		return err
	}

	if cmd.Process != nil {
		if j.Input == "-" {
			return nil
		}
		r := bufio.NewReader(stdout)
		for {
			l, _, _ := r.ReadLine()
			s := string(l)
			if strings.Contains(s, AFL_SUCCESS_MSG) {
				break
			}
			m := regexp.MustCompile(AFL_FAIL_REGEX).FindStringSubmatch(s)
			if len(m) > 0 {
				return errors.New(stripAnsi(m[1]))
			}
		}
	}

	return nil
}

func (j Job) Stop() error {
	processes, err := ps.Processes()
	if err != nil {
		logger.Error(err)
		return err
	}

	for _, p := range processes {
		if p.Executable() == filepath.Base(j.TargetApp) {
			p1, _ := ps.FindProcess(p.Pid())
			p2, _ := ps.FindProcess(p1.PPid())
			p3, _ := ps.FindProcess(p2.PPid())

			killProcess(p1)
			killProcess(p2)
			killProcess(p3)
		}
	}

	return nil
}

func (j Job) View() ([]Stats, error) {
	var stats []Stats

	for c := 1; c <= j.Cores; c++ {
		fuzzerID := fmt.Sprintf("fuzzer%d", c)
		filePath := []string{j.AFLDir, j.Output, fuzzerID, AFL_STATS_FILE}
		fileName := strings.Join(filePath, `\`)

		if !fileExists(fileName) {
			text := fmt.Sprintf("Statistics are unavailable for %s", j.Name)
			err := errors.New(text)
			return stats, err
		}

		content, err := ioutil.ReadFile(fileName)
		if err != nil {
			continue
		}

		newStats, err := parseStats(string(content))
		if err != nil {
			continue
		}

		stats = append(stats, newStats)
	}

	return stats, nil
}

func (j Job) Check(pid int) (bool, error) {
	p, err := ps.FindProcess(pid)
	if err != nil {
		return false, err
	}

	if p != nil && strings.Contains(p.Executable(), "afl-fuzz.exe") {
		return true, nil
	}

	return false, nil
}

func (j Job) Collect() ([]Crash, error) {
	var crashes []Crash

	dirname := path.Join(j.AFLDir, j.Output)
	re := regexp.MustCompile(`\\crashes.*\\id_\d{6}_\d{2}`)
	err := godirwalk.Walk(dirname, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if re.MatchString(osPathname) {
				crashDir := strings.Split(filepath.Dir(osPathname), "\\")
				fuzzerID := crashDir[len(crashDir)-2]
				newCrash := newCrash(j.GUID, fuzzerID, osPathname)
				crashes = append(crashes, newCrash)
			}
			return nil
		},
		Unsorted: true,
	})

	return crashes, err
}

func startJob(c *gin.Context) {
	j := newJob(c.Param("guid"))
	fuzzerID := c.DefaultQuery("fid", "fuzzer1")

	if err := c.ShouldBindJSON(&j); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	if err := j.Start(fuzzerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"guid":  j.GUID,
			"error": err.Error(),
		})
		return
	}

	if ok, _ := project.FindJob(j.GUID); !ok {
		project.AddJob(j)
	}

	c.JSON(http.StatusCreated, gin.H{
		"guid": j.GUID,
		"msg":  fmt.Sprintf("Instance %s of job %s has been successfuly started!", fuzzerID, j.Name),
	})
}

func stopJob(c *gin.Context) {
	j, i, err := project.GetJob(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	if err := j.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"guid":  j.GUID,
			"error": err.Error(),
		})
		return
	}

	project.RemoveJob(i)

	c.JSON(http.StatusOK, gin.H{
		"guid": j.GUID,
		"msg":  fmt.Sprintf("Job %s has been successfully stopped!", j.Name),
	})
}

func viewJob(c *gin.Context) {
	j, _, err := project.GetJob(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	Stats, err := j.View()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  j.GUID,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Stats)
}

func checkJob(c *gin.Context) {
	processIDs := []int{}
	msg := ""

	c.Bind(&processIDs)
	if len(processIDs) < 1 || len(processIDs) > 4 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Invalid number of arguments provided.",
		})
		return
	}

	j, _, err := project.GetJob(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	for _, processID := range processIDs {
		ok, err := j.Check(processID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"msg": fmt.Sprintf("Fuzzer instance with PID %d cannot be found.", processID),
				"pid": processID,
			})
			return
		}
	}

	if len(processIDs) > 1 {
		msg = "All fuzzer instances seem to be up and running."
	} else {
		msg = fmt.Sprintf("Fuzzer instance with PID %d is up and running.", processIDs[0])
	}

	c.JSON(http.StatusOK, gin.H{
		"msg": msg,
	})
}

func collectJob(c *gin.Context) {
	j, _, err := project.GetJob(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	Crashes, err := j.Collect()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  j.GUID,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Crashes)
}

func plotJob(c *gin.Context) {
	j, _, err := project.GetJob(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	fuzzerID := c.Query("fid")
	filePath := filepath.Join(j.AFLDir, j.Output, fuzzerID, AFL_PLOT_FILE)
	// TODO: Add a security check for filepath.
	// if !strings.HasPrefix(filepath.Clean(filePath), "C:\\Tools\\") {
	// 	c.String(403, "Invalid file path!")
	// 	return
	// }

	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+AFL_PLOT_FILE)
	c.Header("Content-Type", "application/octet-stream")
	c.File(filePath)
}
