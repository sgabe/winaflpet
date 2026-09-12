//go:build windows
// +build windows

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/karrick/godirwalk"
	"github.com/mitchellh/go-ps"
	"github.com/rs/xid"
)

const (
	AFL_EXECUTABLE        = "afl-fuzz.exe"
	AFL_SUCCESS_MSG       = "All set and ready to roll!"
	AFL_FAIL_REGEX        = `(?:PROGRAM ABORT|OS message) : (.*)`
	AFL_STATS_FILE        = "fuzzer_stats"
	AFL_PLOT_FILE         = "plot_data"
	MAX_ZIP_BYTES   int64 = 256 << 20
	MAX_EXP_BYTES   int64 = 1 << 30
)

type inputUploadResult struct {
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Directory string `json:"directory"`
}

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
	PostLib        string `json:"post_lib"`
	PostLibArgs    string `json:"post_lib_args"`
	MemoryLimit    string `json:"memory_limit"`
	PersistCache   int    `json:"persist_cache"`
	DirtyMode      int    `json:"dirty_mode"`
	DumbMode       int    `json:"dumb_mode"`
	CrashMode      int    `json:"crash_mode"`
	BugBucket      int    `json:"bug_bucket"`
	ExpertMode     int    `json:"expert_mode"`
	VariableMode   int    `json:"variable_mode"`
	SequentialMode int    `json:"sequential_mode"`
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

func (j Job) GetCmd(fID int) (*exec.Cmd, error) {
	fuzzerID := fmt.Sprintf("%s%d", j.Banner, fID)

	binDir := "bin32"
	if j.TargetArch == "x64" {
		binDir = "bin64"
	}

	j.AFLDir = path.Join(j.AFLDir, binDir)
	j.DrioDir = path.Join(j.DrioDir, binDir)

	if j.VariableMode != 0 {
		j.CoverageType = []string{"edge", "bb"}[rand.Intn(2)]
		j.FuzzIter = int(float64(j.FuzzIter) * (1 + (rand.Float64() - 0.5)))
	}

	afl, err := exec.LookPath(path.Join(j.AFLDir, AFL_EXECUTABLE))
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	targetCmd, targetArgs := splitCmdLine(j.TargetApp)
	if j.SequentialMode != 0 {
		targetCmd = sequentialName(targetCmd, fID)
		j.TargetModule = sequentialName(j.TargetModule, fID)
	}

	targetApp, err := exec.LookPath(targetCmd)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	envs := os.Environ()

	if j.Autoresume != 0 {
		envs = append(envs, "AFL_AUTORESUME=1")
	} else {
		fuzzerDir := joinPath(j.AFLDir, j.Output, fuzzerID)
		statsFile := joinPath(fuzzerDir, AFL_STATS_FILE)
		if fileExists(statsFile) {
			err := os.RemoveAll(fuzzerDir)
			if err != nil {
				logger.Error(err)
				return nil, err
			}
		}
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
	if j.Cores > 1 && fID == 1 {
		opRole = "-M"
	}

	args = append(args, fmt.Sprintf("%s %s", opRole, fuzzerID))
	args = append(args, fmt.Sprintf("-i %s", j.Input))
	args = append(args, fmt.Sprintf("-o %s", j.Output))

	if j.InstMode == "TinyInst" {
		args = append(args, "-y")
	} else {
		args = append(args, fmt.Sprintf("-D %s", j.DrioDir))
	}

	timeoutSuffix := ""
	if j.Autoresume != 0 || j.Input == "-" {
		timeoutSuffix = "+" // Skip queue entries that time out.
	}

	args = append(args, fmt.Sprintf("-t %d%s", j.Timeout, timeoutSuffix))

	if j.InstMode == "DynamoRIO" {
		if j.PersistCache != 0 {
			args = append(args, "-p")
		}
		if j.ExpertMode != 0 {
			args = append(args, "-e")
		}
	}

	if j.DirtyMode != 0 {
		args = append(args, "-d")
	}

	if j.BugBucket != 0 {
		args = append(args, "-b")
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

	if j.PostLib != "" {
		envs = append(envs, fmt.Sprintf("AFL_POST_LIBRARY=%s", j.PostLib))
		if j.PostLibArgs != "" {
			envs = append(envs, fmt.Sprintf("AFL_POST_LIBRARY_ARGS=%s", j.PostLibArgs))
		}
	}

	if j.ExtrasDir != "" {
		args = append(args, fmt.Sprintf("-x %s", j.ExtrasDir))
	}

	args = append(args, "--")
	args = append(args, fmt.Sprintf("-covtype %s", j.CoverageType))

	for _, m := range strings.Split(j.CoverageModule, ",") {
		if j.InstMode == "TinyInst" {
			args = append(args, fmt.Sprintf("-instrument_module %s", m))
		} else {
			args = append(args, fmt.Sprintf("-coverage_module %s", m))
		}
	}

	if j.InstMode == "TinyInst" {
		args = append(args, "-persist")
		args = append(args, "-loop")
		args = append(args, fmt.Sprintf("-iterations %d", j.FuzzIter))
	} else {
		args = append(args, fmt.Sprintf("-fuzz_iterations %d", j.FuzzIter))
	}

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
	return cmd, nil
}

func (j Job) View() ([]Stats, []string, error) {
	var stats []Stats
	var missing []string

	for c := 1; c <= j.Cores; c++ {
		fuzzerID := fmt.Sprintf("%s%d", j.Banner, c)
		fileName := joinPath(j.AFLDir, j.Output, fuzzerID, AFL_STATS_FILE)

		if !fileExists(fileName) {
			missing = append(missing, fmt.Sprintf("Statistics are unavailable for fuzzer instance #%d in job %s.", c, j.Name))
			continue
		}

		content, err := os.ReadFile(fileName)
		if err != nil {
			missing = append(missing,
				fmt.Sprintf("Statistics are unreadable for fuzzer instance #%d in job %s.", c, j.Name))
			continue
		}

		newStats, err := parseStats(string(content))
		if err != nil {
			missing = append(missing,
				fmt.Sprintf("Statistics are invalid for fuzzer instance #%d in job %s.", c, j.Name))
			continue
		}

		stats = append(stats, newStats)
	}

	return stats, missing, nil
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

	outputDir := joinPath(j.AFLDir, j.Output)
	re := regexp.MustCompile(`\\crashes(_\d{14})?\\id_\d{6}_\w+$`)
	err := godirwalk.Walk(outputDir, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if !re.MatchString(osPathname) {
				return nil
			}

			fileHash, err := hashFile(osPathname)
			if err != nil {
				return err
			}

			crashPath := joinPath(outputDir, "crashes", fileHash)
			if err := copyFile(osPathname, crashPath); err != nil {
				return err
			}

			crashDir := strings.Split(filepath.Dir(osPathname), "\\")
			fuzzerID := crashDir[len(crashDir)-2]
			funcAddr := getFuncAddr(osPathname)
			newCrash := newCrash(j.GUID, fuzzerID, funcAddr, crashPath)
			crashes = append(crashes, newCrash)

			return nil
		},
		Unsorted: true,
	})

	return crashes, err
}

func startJob(c *gin.Context) {
	var j Job
	guid, err := xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fid, err := strconv.Atoi(c.DefaultQuery("fid", "0"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid instance ID"})
		return
	}
	if err = c.ShouldBindJSON(&j); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if j.GUID != guid || j.Cores < 1 || j.Cores > 40 || fid < 0 || fid > j.Cores || strings.TrimSpace(j.TargetApp) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job, target or instance selection"})
		return
	}
	requestID := c.Query("request_id")
	if _, err = xid.FromString(requestID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A valid request_id is required"})
		return
	}
	snapshot, err := supervisor.start(j, fid, requestID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, snapshot)
}

func stopJob(c *gin.Context) {
	guid, err := xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var request struct {
		Job        Job      `json:"job"`
		RequestIDs []string `json:"request_ids"`
	}
	if err = c.ShouldBindJSON(&request); err != nil || request.Job.GUID != guid || request.Job.Cores < 1 || request.Job.Cores > 40 || len(request.RequestIDs) > 41 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid stop request"})
		return
	}
	for _, id := range request.RequestIDs {
		if _, err = xid.FromString(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
			return
		}
	}
	snapshot, err := supervisor.stopRequests(request.Job, request.RequestIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, snapshot)
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

	stats, missing, err := j.View()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"guid":  j.GUID,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":   stats,
		"missing": missing,
	})
}

func checkJob(c *gin.Context) {
	guid, err := xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, supervisor.snapshot(guid))
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

func inputJob(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MAX_ZIP_BYTES+(1<<20))
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid upload or ZIP exceeds 256 MiB"})
		return
	}
	var j Job
	guid, err := xid.FromString(c.Param("guid"))
	if err != nil || json.Unmarshal([]byte(c.PostForm("job")), &j) != nil || j.GUID != guid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job configuration"})
		return
	}
	supervisor.mu.Lock()
	busy := supervisor.shuttingDown
	if job := supervisor.jobs[guid]; job != nil {
		for _, instance := range job.instances {
			busy = busy || active(instance.state.Status)
		}
	}
	supervisor.mu.Unlock()
	if busy {
		c.JSON(http.StatusConflict, gin.H{"error": "Agent is shutting down or the job is active"})
		return
	}
	header, err := c.FormFile("input")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Select a ZIP file"})
		return
	}
	if header.Size <= 0 || header.Size > MAX_ZIP_BYTES {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ZIP must be between 1 byte and 256 MiB"})
		return
	}
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()
	archive, err := zip.NewReader(file, header.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid ZIP: %s", err)})
		return
	}
	result := inputUploadResult{Directory: j.Input}
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > uint64(MAX_EXP_BYTES-result.Bytes) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ZIP expands beyond 1 GiB"})
			return
		}
		result.Files++
		result.Bytes += int64(entry.UncompressedSize64)
	}
	if result.Files == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ZIP contains no files"})
		return
	}
	// ReadDir also checks that Input exists and is a directory.
	entries, err := os.ReadDir(j.Input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, entry := range entries {
		if err := c.Request.Context().Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !entry.IsDir() {
			if err := os.Remove(filepath.Join(j.Input, entry.Name())); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
	}
	if err := os.CopyFS(j.Input, archive); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Input upload failed: %s", err)})
		return
	}
	c.JSON(http.StatusOK, result)
}
