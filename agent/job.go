//go:build windows
// +build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-ps"
	"github.com/rs/xid"
)

const (
	AFL_EXECUTABLE  = "afl-fuzz.exe"
	AFL_SUCCESS_MSG = "All set and ready to roll!"
	AFL_FAIL_REGEX  = `(?:PROGRAM ABORT|OS message) : (.*)`
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
	DelivMode      string `json:"deliv_mode"`
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
	ExtrasDir      string `json:"extras_dir"`
	AttachLib      string `json:"attach_lib"`
	CustomLib      string `json:"custom_lib"`
	MemoryLimit    string `json:"memory_limit"`
	PersistCache   int    `json:"persist_cache"`
	DirtyMode      int    `json:"dirty_mode"`
	DumbMode       int    `json:"dumb_mode"`
	CrashMode      int    `json:"crash_mode"`
	ExpertMode     int    `json:"expert_mode"`
	NoAffinity     int    `json:"no_affinity"`
	SkipCrashes    int    `json:"skip_crashes"`
	ShuffleQueue   int    `json:"shuffle_queue"`
	Autoresume     int    `json:"autoresume"`
	Status         int    `json:"status"`
}

func newJob(GUID string) Job {
	j := new(Job)
	j.Cores = 1
	j.GUID, _ = xid.FromString(GUID)
	return *j
}

func (j Job) Start(fID int) error {
	afl, err := exec.LookPath(path.Join(j.AFLDir, AFL_EXECUTABLE))
	if err != nil {
		logger.Error(err)
		return err
	}

	targetCmd, targetArgs := splitCmdLine(j.TargetApp)
	targetApp, err := exec.LookPath(targetCmd)
	if err != nil {
		logger.Error(err)
		return err
	}

	envs := os.Environ()

	if j.Autoresume != 0 {
		envs = append(envs, "AFL_AUTORESUME=1")
	}

	if j.SkipCrashes != 0 || j.Autoresume != 0 {
		envs = append(envs, "AFL_SKIP_CRASHES=1")
	}

	if j.ShuffleQueue != 0 {
		envs = append(envs, "AFL_SHUFFLE_QUEUE=1")
	}

	if j.NoAffinity != 0 {
		envs = append(envs, "AFL_NO_AFFINITY=1")
	}

	args := []string{}

	if j.DelivMode == "sm" {
		args = append(args, "-s")
		targetArgs += "-s @@"
	} else {
		targetArgs += "-f @@"
	}

	opRole := "-S"
	fuzzerID := fmt.Sprintf("%s%d", j.Banner, fID)
	if j.Cores > 1 && fID == 1 {
		opRole = "-M"
	}

	args = append(args, fmt.Sprintf("%s %s", opRole, fuzzerID))
	args = append(args, fmt.Sprintf("-i %s", j.Input))
	args = append(args, fmt.Sprintf("-o %s", j.Output))
	args = append(args, fmt.Sprintf("-D %s", j.DrioDir))

	timeoutSuffix := ""
	if j.Autoresume != 0 || j.Input == "-" {
		timeoutSuffix = "+" // Skip queue entries that time out.
	}

	args = append(args, fmt.Sprintf("-t %d%s", j.Timeout, timeoutSuffix))

	if j.PersistCache != 0 {
		args = append(args, "-p")
	}

	if j.DirtyMode != 0 {
		args = append(args, "-d")
	}

	if j.ExpertMode != 0 {
		args = append(args, "-e")
	}

	if j.CrashMode != 0 && j.DumbMode == 0 {
		args = append(args, "-C")
	}

	if j.DumbMode != 0 && j.CrashMode == 0 {
		args = append(args, "-n")
	}

	if j.MemoryLimit != "0" && j.MemoryLimit != "" {
		args = append(args, fmt.Sprintf("-m %s", j.MemoryLimit))
	}

	if j.AttachLib != "" {
		args = append(args, fmt.Sprintf("-A %s", j.AttachLib))
	}

	if j.CustomLib != "" {
		args = append(args, fmt.Sprintf("-l %s", j.CustomLib))
	}

	if j.ExtrasDir != "" {
		args = append(args, fmt.Sprintf("-x %s", j.ExtrasDir))
	}

	args = append(args, "--")
	args = append(args, fmt.Sprintf("-covtype %s", j.CoverageType))

	for _, m := range strings.Split(j.CoverageModule, ",") {
		args = append(args, fmt.Sprintf("-coverage_module %s", m))
	}

	args = append(args, fmt.Sprintf("-fuzz_iterations %d", j.FuzzIter))
	args = append(args, fmt.Sprintf("-target_module %s", j.TargetModule))
	args = append(args, fmt.Sprintf("-target_method %s", j.TargetMethod))
	args = append(args, fmt.Sprintf("-target_offset %s", j.TargetOffset))
	args = append(args, fmt.Sprintf("-nargs %d", j.TargetNArgs))
	args = append(args, "--")
	args = append(args, targetApp)
	args = append(args, targetArgs)

	cmd := exec.Command(afl, strings.Join(args, " "))
	cmd.Dir = j.AFLDir
	cmd.Env = envs
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.CmdLine = strings.Join(cmd.Args, ` `)
	stdoutPipe, _ := cmd.StdoutPipe()
	stdoutReader := bufio.NewReader(stdoutPipe)

	if err := cmd.Start(); err != nil {
		logger.Error(err)
		return err
	}

	c := make(chan error)

	go readStdout(c, stdoutReader)

	select {
	case err := <-c:
		return err
	case <-time.After(4 * time.Minute):
		return nil
	}
}

func (j Job) Stop() error {
	processes, err := ps.Processes()
	if err != nil {
		logger.Error(err)
		return err
	}

	targetCmd, _ := splitCmdLine(j.TargetApp)
	targetExe := filepath.Base(targetCmd)
	targetProcs := []ps.Process{}

	for _, p := range processes {
		if p.Executable() == targetExe {
			p1, _ := ps.FindProcess(p.Pid())
			if p1 != nil {
				targetProcs = append(targetProcs, p1)
				p2, _ := ps.FindProcess(p1.PPid())
				if p2 != nil {
					targetProcs = append(targetProcs, p2)
					p3, _ := ps.FindProcess(p2.PPid())
					if p3 != nil {
						targetProcs = append(targetProcs, p3)
					}
				}
			}
		}
	}

	for _, p := range targetProcs {
		killProcess(p)
	}

	return nil
}

func (j Job) View() ([]Stats, error) {
	var stats []Stats

	for c := 1; c <= j.Cores; c++ {
		fuzzerID := fmt.Sprintf("%s%d", j.Banner, c)
		fileName := joinPath(j.AFLDir, j.Output, fuzzerID, AFL_STATS_FILE)

		if !fileExists(fileName) {
			text := fmt.Sprintf("Statistics are unavailable for fuzzer instance #%d in job %s", c, j.Name)
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

	dirname := joinPath(j.AFLDir, j.Output)
	re := regexp.MustCompile(`\\crashes(_\d{14})?\\id_\d{6}_\w+$`)
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

	fID, err := strconv.Atoi(c.DefaultQuery("fid", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	if err := c.ShouldBindJSON(&j); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	if err := j.Start(fID); err != nil {
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
		"msg":  fmt.Sprintf("Fuzzer instance #%d of job %s has been successfuly started!", fID, j.Name),
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
	if len(processIDs) < 1 || len(processIDs) > 40 {
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

	fID, err := strconv.Atoi(c.Query("fid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"guid":  c.Param("guid"),
			"error": err.Error(),
		})
		return
	}

	fuzzerID := fmt.Sprintf("%s%d", j.Banner, fID)
	filePath := joinPath(j.AFLDir, j.Output, fuzzerID, AFL_PLOT_FILE)
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
