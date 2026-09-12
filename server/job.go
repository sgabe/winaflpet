package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
	"github.com/rs/xid"
	"github.com/sgabe/structable"
)

const (
	TB_NAME_JOBS   = "jobs"
	TB_SCHEMA_JOBS = `CREATE TABLE jobs (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"aid" INTEGER,
		"guid" TEXT NOT NULL UNIQUE,
		"name" TEXT,
		"desc" TEXT,
		"banner" TEXT NOT NULL,
		"cores" INTEGER NOT NULL,
		"input" TEXT NOT NULL,
		"output" TEXT NOT NULL,
		"timeout" INTEGER NOT NULL,
		"inst_mode" TEXT NOT NULL,
		"deliv_mode" TEXT,
		"cov_type" TEXT NOT NULL,
		"cov_module" TEXT NOT NULL,
		"fuzz_iter" INTEGER NOT NULL,
		"target_module" TEXT NOT NULL,
		"target_method" TEXT,
		"target_offset" TEXT,
		"target_nargs" INTEGER,
		"target_app" TEXT NOT NULL,
		"target_arch" TEXT NOT NULL,
		"afl_dir" TEXT NOT NULL,
		"drio_dir" TEXT NOT NULL,
		"py_dir" TEXT NOT NULL,
		"bugid_dir" TEXT NOT NULL,
		"extras_dir" TEXT,
		"attach_lib" TEXT,
		"custom_lib" TEXT,
		"post_lib" TEXT,
		"post_lib_args" TEXT,
		"memory_limit" TEXT,
		"persist_cache" INTEGER,
		"dirty_mode" INTEGER,
		"dumb_mode" INTEGER,
		"crash_mode" INTEGER,
		"bug_bucket" INTEGER,
		"expert_mode" INTEGER,
		"variable_mode" INTEGER,
		"sequential_mode" INTEGER,
		"skip_crashes" INTEGER,
		"autoresume" INTEGER,
		"shuffle_queue" INTEGER,
		"no_affinity" INTEGER,
		"status" INTEGER,
		FOREIGN KEY (aid) REFERENCES agents(id)
	  );`
)

type Job struct {
	structable.Recorder `json:"-"`

	ID             int    `stbl:"id, PRIMARY_KEY, AUTO_INCREMENT"`
	AgentID        int    `json:"aid" form:"aid" stbl:"aid"`
	GUID           xid.ID `json:"guid" stbl:"guid"`
	Name           string `json:"name" form:"name" stbl:"name"`
	Desc           string `json:"desc" form:"desc" stbl:"desc"`
	Banner         string `json:"banner" form:"banner" stbl:"banner"`
	Cores          int    `json:"cores" form:"cores" stbl:"cores"`
	Input          string `json:"input" form:"input" stbl:"input"`
	Output         string `json:"output" form:"output" stbl:"output"`
	Timeout        int    `json:"timeout" form:"timeout" stbl:"timeout"`
	InstMode       string `json:"inst_mode" form:"inst_mode" stbl:"inst_mode"`
	DelivMode      string `json:"deliv_mode" form:"deliv_mode" stbl:"deliv_mode"`
	CoverageType   string `json:"cov_type" form:"cov_type" stbl:"cov_type"`
	CoverageModule string `json:"cov_module" form:"cov_module" stbl:"cov_module"`
	FuzzIter       int    `json:"fuzz_iter" form:"fuzz_iter" stbl:"fuzz_iter"`
	TargetModule   string `json:"target_module" form:"target_module" stbl:"target_module"`
	TargetMethod   string `json:"target_method" form:"target_method" stbl:"target_method"`
	TargetOffset   string `json:"target_offset" form:"target_offset" stbl:"target_offset"`
	TargetNArgs    int    `json:"target_nargs" form:"target_nargs" stbl:"target_nargs"`
	TargetApp      string `json:"target_app" form:"target_app" stbl:"target_app"`
	TargetArch     string `json:"target_arch" form:"target_arch" stbl:"target_arch"`
	AFLDir         string `json:"afl_dir" form:"afl_dir" stbl:"afl_dir"`
	DrioDir        string `json:"drio_dir" form:"drio_dir" stbl:"drio_dir"`
	PyDir          string `json:"py_dir" form:"py_dir" stbl:"py_dir"`
	BugIdDir       string `json:"bugid_dir" form:"bugid_dir" stbl:"bugid_dir"`
	ExtrasDir      string `json:"extras_dir" form:"extras_dir" stbl:"extras_dir"`
	AttachLib      string `json:"attach_lib" form:"attach_lib" stbl:"attach_lib"`
	CustomLib      string `json:"custom_lib" form:"custom_lib" stbl:"custom_lib"`
	PostLib        string `json:"post_lib" form:"post_lib" stbl:"post_lib"`
	PostLibArgs    string `json:"post_lib_args" form:"post_lib_args" stbl:"post_lib_args"`
	MemoryLimit    string `json:"memory_limit" form:"memory_limit" stbl:"memory_limit"`
	PersistCache   int    `json:"persist_cache" form:"persist_cache" stbl:"persist_cache"`
	DirtyMode      int    `json:"dirty_mode" form:"dirty_mode" stbl:"dirty_mode"`
	DumbMode       int    `json:"dumb_mode" form:"dumb_mode" stbl:"dumb_mode"`
	CrashMode      int    `json:"crash_mode" form:"crash_mode" stbl:"crash_mode"`
	BugBucket      int    `json:"bug_bucket" form:"bug_bucket" stbl:"bug_bucket"`
	ExpertMode     int    `json:"expert_mode" form:"expert_mode" stbl:"expert_mode"`
	VariableMode   int    `json:"variable_mode" form:"variable_mode" stbl:"variable_mode"`
	SequentialMode int    `json:"sequential_mode" form:"sequential_mode" stbl:"sequential_mode"`
	NoAffinity     int    `json:"no_affinity" form:"no_affinity" stbl:"no_affinity"`
	SkipCrashes    int    `json:"skip_crashes" form:"skip_crashes" stbl:"skip_crashes"`
	ShuffleQueue   int    `json:"shuffle_queue" form:"shuffle_queue" stbl:"shuffle_queue"`
	Autoresume     int    `json:"autoresume" form:"autoresume" stbl:"autoresume"`
	Status         status `json:"status" form:"status" stbl:"status"`
}

func newJob() *Job {
	j := new(Job)
	j.GUID = xid.New()
	j.Cores = 1
	j.InstMode = "DynamoRIO"
	j.CrashMode = 0
	j.DirtyMode = 0
	j.DumbMode = 0
	j.PersistCache = 0
	j.BugBucket = 0
	j.ExpertMode = 0
	j.VariableMode = 0
	j.SequentialMode = 0
	j.NoAffinity = 0
	j.SkipCrashes = 0
	j.ShuffleQueue = 0
	j.Autoresume = 0
	j.Recorder = structable.New(db, DB_FLAVOR).Bind(TB_NAME_JOBS, j)
	return j
}

func (j *Job) LoadByGUID() error {
	return j.Recorder.LoadWhere("guid = ?", j.GUID)
}

func (j *Job) GetProcessIDs(fID int) ([]int, error) {
	var processIDs []int

	if fID != 0 {
		s := newStat()
		s.JobID = j.ID
		s.AFLBanner = fmt.Sprintf("%s%d", j.Banner, fID)
		if err := s.LoadJobIDFuzzerID(); err != nil {
			return processIDs, err
		}
		return []int{s.FuzzerProcessID}, nil
	}

	for c := 1; c <= j.Cores; c++ {
		s := newStat()
		s.JobID = j.ID
		s.AFLBanner = fmt.Sprintf("%s%d", j.Banner, c)
		if err := s.LoadJobIDFuzzerID(); err != nil {
			return processIDs, err
		}
		processIDs = append(processIDs, s.FuzzerProcessID)
	}

	return processIDs, nil
}

func (j *Job) Cleanup(fID int) error {
	fuzzerID := fmt.Sprintf("%s%d", j.Banner, fID)
	crashes := squirrel.Select("id").From(TB_NAME_CRASHES).Where(squirrel.Eq{"jid": j.ID}, squirrel.Eq{"fuzzerid": fuzzerID})
	rows, err := crashes.RunWith(db).Query()
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		c := newCrash()
		if err := rows.Scan(&c.ID); err != nil {
			return err
		}
		if err := c.Load(); err != nil {
			return err
		}
		if strings.Contains(c.Args, "\\crashes\\") {
			c.Delete()
		}
	}

	return nil
}

func (j *Job) GetAgent() (*Agent, error) {
	a := newAgent()
	a.ID = j.AgentID
	if err := a.Load(); err != nil {
		return a, err
	}
	return a, nil
}

func (j *Job) HasAlert() bool {
	if ok, _ := alert.FindJob(j.GUID); ok {
		return true
	}
	return false
}

func loadJobs() ([]*Job, error) {
	j := &Job{}
	sj := structable.New(db, DB_FLAVOR).Bind(TB_NAME_JOBS, j)

	fn := func(d structable.Describer, q squirrel.SelectBuilder) (squirrel.SelectBuilder, error) {
		return q.Limit(10), nil
	}

	items, err := listWhere(sj, fn)
	if err != nil {
		return []*Job{}, err
	}

	// Because we get back a []Recorder, we need to get the original data
	// back out. We have to manually convert it back to its real type.
	jobs := make([]*Job, len(items))
	for i, item := range items {
		jobs[i] = item.Interface().(*Job)
	}

	return jobs, err
}

const maxInputZIPBytes int64 = 256 << 20

type inputUploadResult struct {
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Directory string `json:"directory"`
}

// Stream the stored upload with server-loaded job settings. No destination from
// the browser or the ZIP is trusted, and uploads are never automatically retried.
func sendInputZIP(ctx context.Context, url, key string, job interface{}, archive io.Reader, size int64) (result inputUploadResult, err error) {
	if size <= 0 || size > maxInputZIPBytes {
		return result, errors.New("ZIP must be between 1 byte and 256 MiB")
	}
	config, err := json.Marshal(job)
	if err != nil {
		return result, err
	}
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	if err = writer.WriteField("job", string(config)); err != nil {
		return result, err
	}
	if _, err = writer.CreateFormFile("input", "input.zip"); err != nil {
		return result, err
	}
	prefix := append([]byte(nil), framing.Bytes()...)
	framing.Reset()
	if err = writer.Close(); err != nil {
		return result, err
	}
	body := io.MultiReader(bytes.NewReader(prefix), io.LimitReader(archive, size), bytes.NewReader(framing.Bytes()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return result, err
	}
	req.ContentLength = int64(len(prefix)) + size + int64(framing.Len())
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Auth-Key", key)
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("Input upload unconfirmed: %w. Check the agent's Input directory before retrying", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&failure)
		if resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusNotFound {
			return result, errors.New("This agent does not support input uploads; update the agent")
		}
		return result, fmt.Errorf("Agent rejected input upload (HTTP %d): %s", resp.StatusCode, failure.Error)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil || result.Files <= 0 || result.Directory == "" {
		return result, errors.New("Input upload unconfirmed: invalid agent response. Check the agent's Input directory before retrying")
	}
	return result, nil
}

func (j *Job) confirmInputUpload() error {
	if strings.TrimSpace(j.Input) == "" || j.Input == "-" {
		return errors.New("Set and save an Input directory before uploading. '-' is a resume marker, not a directory.")
	}
	if err := j.allowConfigChange(); err != nil {
		return fmt.Errorf("Cannot upload while the job is running or its state is unconfirmed: %w", err)
	}
	return nil
}

func startJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	j := newJob()
	var err error
	j.GUID, err = xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = j.LoadByGUID(); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	fid, err := strconv.Atoi(c.DefaultQuery("fid", "0"))
	if err != nil || fid < 0 || fid > j.Cores || j.Cores < 1 || j.Cores > 40 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid instance selection"})
		return
	}
	// Refuse delivery to an older agent; it cannot safely deduplicate retries.
	var capability JobSnapshot
	if err = agentRequest(j, "check", nil, &capability); err != nil || capability.Protocol != 2 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Start not submitted: a reachable lifecycle protocol 2 agent is required"})
		return
	}
	if !capability.Known {
		if err = j.allowUnknownRun(); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
	}
	requestID, payload, err := j.pendingStart(fid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var snapshot JobSnapshot
	err = agentRequest(j, fmt.Sprintf("start?fid=%d&request_id=%s", fid, requestID), payload, &snapshot)
	if err != nil {
		var responseError *agentHTTPError
		if errors.As(err, &responseError) && responseError.Code >= 400 && responseError.Code < 500 {
			_ = j.clearPendingStart(fid)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "Start unconfirmed: " + err.Error()})
		return
	}
	if err = validateSnapshot(snapshot, payload.Cores); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Start unconfirmed: " + err.Error()})
		return
	}
	snapshot.ObservedAt = time.Now().Unix()
	if err = j.saveSnapshot(snapshot); err == nil {
		err = j.clearPendingStart(fid)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"alert": snapshot.Msg, "snapshot": snapshot, "request_id": requestID, "context": "success"})
}

func stopJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	j := newJob()
	var err error
	j.GUID, err = xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = j.LoadByGUID(); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var snapshot JobSnapshot
	ids, err := j.pendingStartIDs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err = agentRequest(j, "stop", gin.H{"job": j, "request_ids": ids}, &snapshot); err == nil {
		err = validateSnapshot(snapshot, j.Cores)
	}
	if err != nil {
		previous, _ := j.snapshot()
		c.JSON(http.StatusBadGateway, gin.H{"error": "Stop unconfirmed: " + err.Error(), "snapshot": previous, "stale": true})
		return
	}
	snapshot.ObservedAt = time.Now().Unix()
	if err = j.saveSnapshot(snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stop response could not be saved: " + err.Error()})
		return
	}
	if _, err = db.Exec("DELETE FROM job_start_requests WHERE guid=?", j.GUID.String()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stop accepted, but pending request cancellation could not be saved: " + err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"alert": snapshot.Msg, "snapshot": snapshot, "context": "info"})
}

func deleteJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if err := j.allowConfigChange(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}
	if err := j.deleteConfiguration(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   fmt.Sprintf("Job %s has been successfully deleted!", j.Name),
		"context": "success",
	})
}

func viewJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{
			"alert":    err.Error(),
			"template": "job_view",
		})
		return
	}

	title := fmt.Sprintf("Job %s", j.Name)
	// TODO: Handle errors.
	a, _ := j.GetAgent()

	request := gorequest.New()
	request.Debug = false

	type StatsTemp struct {
		Stats   []Stat   `json:"stats"`
		Missing []string `json:"missing"`
	}

	var statsTemp StatsTemp

	targetURL := fmt.Sprintf("http://%s:%d/job/%s/view", a.Host, a.Port, j.GUID)
	resp, _, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).EndStruct(&statsTemp)
	if errs != nil {
		otherError(c, map[string]string{
			"title":    title,
			"alert":    fmt.Sprintf("Stats are not yet available for job %s.", j.Name),
			"template": "job_view",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		otherError(c, map[string]string{
			"title":    title,
			"alert":    "Job not found on the remote host!",
			"template": "job_view",
		})
		return
	}

	var stats []Stat
	for _, stat := range statsTemp.Stats {
		s := newStat()
		s.JobID = j.ID
		s.AFLBanner = stat.AFLBanner
		if ok, _ := s.ExistsWhere("jid = ? and afl_banner = ?", s.JobID, s.AFLBanner); ok {
			s.LoadJobIDFuzzerID()
			s.CopyStat(stat)
			s.Update()
		} else {
			s.CopyStat(stat)
			s.Insert()
		}
		stats = append(stats, *s)
	}

	alert := ""
	context := ""

	if len(statsTemp.Missing) > 0 {
		alert = strings.Join(statsTemp.Missing, "<br/>")
		context = "warning"
	}

	c.HTML(http.StatusOK, "job_view", gin.H{
		"title":   title,
		"stats":   stats,
		"alert":   alert,
		"context": context,
		"path":    c.Request.URL.Path,
	})
}

func plotJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		c.HTML(http.StatusOK, "job_plot", gin.H{
			"alert":   err.Error(),
			"context": "danger",
		})
		return
	}

	fID, err := strconv.Atoi(c.Query("fid"))
	if err != nil {
		c.HTML(http.StatusOK, "job_plot", gin.H{
			"alert":   err.Error(),
			"context": "danger",
		})
		return
	}

	jobGUID := j.GUID.String()
	fuzzerID := fmt.Sprintf("%s%d", j.Banner, fID)
	title := fmt.Sprintf("Stats for fuzzer instance #%d in job %s", fID, j.Name)

	switch c.Request.Method {
	case http.MethodGet:
		plots, err := collectPlots(jobGUID, fuzzerID)
		if err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "job_plot",
			})
			return
		}
		c.HTML(http.StatusOK, "job_plot", gin.H{
			"title": title,
			"plots": plots,
			"path":  c.Request.URL.Path,
		})
	case http.MethodPost:
		// TODO: Handle errors.
		a, _ := j.GetAgent()

		request := gorequest.New()
		request.Debug = false

		targetURL := fmt.Sprintf("http://%s:%d/job/%s/plot?fid=%d", a.Host, a.Port, jobGUID, fID)
		resp, bodyBytes, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).EndBytes()
		if errs != nil {
			otherError(c, map[string]string{"alert": errs[0].Error()})
			return
		}

		if resp.StatusCode != http.StatusOK {
			otherError(c, map[string]string{
				"alert":    "Job not found on the remote host!",
				"template": "job_plot",
			})
			return
		}

		if len(bodyBytes) == 0 {
			otherError(c, map[string]string{
				"alert":   fmt.Sprintf("Plot data is not yet available for fuzzer instance #%d in job %s.", fID, j.Name),
				"context": "info",
			})
			return
		}

		if err := savePlotData(jobGUID, fuzzerID, bodyBytes); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}

		if err := createPlots(jobGUID, fuzzerID); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"alert":   fmt.Sprintf("Plot data available for fuzzer instance #%d in job %s.", fID, j.Name),
			"context": "success",
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}

func checkJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	j := newJob()
	var err error
	j.GUID, err = xid.FromString(c.Param("guid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = j.LoadByGUID(); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var snapshot JobSnapshot
	if err = agentRequest(j, "check", nil, &snapshot); err != nil {
		previous, _ := j.snapshot()
		kind := "status_unavailable"
		var agentErr *agentHTTPError
		if errors.As(err, &agentErr) && agentErr.Code == http.StatusNotFound && agentErr.Message == "Job not found" {
			kind = "job_missing"
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "error_kind": kind, "snapshot": previous, "stale": true, "can_edit": j.CanChangeConfig()})
		return
	}
	if snapshot.Protocol == 2 && !snapshot.Known {
		if err = j.allowUnknownRun(); err != nil {
			previous, _ := j.snapshot()
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "error_kind": "run_unrecognized", "snapshot": previous, "stale": true, "can_edit": j.CanChangeConfig()})
			return
		}
		snapshot.Msg = "Ready to start"
		for id := 1; id <= j.Cores; id++ {
			snapshot.Instances = append(snapshot.Instances, InstanceState{FID: id, Status: "stopped"})
		}
		c.JSON(http.StatusOK, gin.H{"snapshot": snapshot, "alert": snapshot.Msg, "stale": false, "can_edit": j.CanChangeConfig()})
		return
	}
	if snapshot, err = resizeIdleSnapshot(snapshot, j.Cores); err != nil {
		previous, _ := j.snapshot()
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "snapshot": previous, "stale": true, "can_edit": j.CanChangeConfig()})
		return
	}
	snapshot.ObservedAt = time.Now().Unix()
	if err = j.saveSnapshot(snapshot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"snapshot": snapshot, "alert": snapshot.Msg, "context": "info", "stale": false, "can_edit": j.CanChangeConfig()})
}

func collectJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	// TODO: Handle errors.
	a, _ := j.GetAgent()

	request := gorequest.New()
	request.Debug = false

	var crashesTemp []Crash
	targetURL := fmt.Sprintf("http://%s:%d/job/%s/collect", a.Host, a.Port, j.GUID)
	resp, _, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).EndStruct(&crashesTemp)
	if errs != nil {
		otherError(c, map[string]string{
			"alert": errs[0].Error(),
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		otherError(c, map[string]string{
			"alert": fmt.Sprintf("Job %s is not found on the remote host!", j.Name),
		})
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

	c.JSON(http.StatusOK, gin.H{
		"alert":   fmt.Sprintf("Found %d new crashes for job %s", len(crashes), j.Name),
		"context": "info",
	})
}

func createJobs(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		// TODO: Handle errors.
		agents, _ := loadAgents()
		c.HTML(http.StatusOK, "jobs_create", gin.H{
			"title":  "Create job",
			"agents": agents,
			"path":   c.Request.URL.Path,
		})
		return
	case http.MethodPut:
		j := newJob()
		if err := c.ShouldBind(&j); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		if err := j.Insert(); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"alert":   fmt.Sprintf("Job %s has been successfully created!", j.Name),
			"context": "success",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}

func uploadJobs(c *gin.Context) {
	var err error
	var f []byte
	var r io.Reader
	var fh *multipart.FileHeader

	j := newJob()
	title := "Upload job"

	switch c.Request.Method {
	case http.MethodGet:
		c.HTML(http.StatusOK, "jobs_upload", gin.H{
			"title": title,
			"path":  c.Request.URL.Path,
		})
		return
	case http.MethodPost:
		fh, err = c.FormFile("job")
		if err != nil {
			break
		}

		r, err = fh.Open()
		if err != nil {
			break
		}

		f, err = io.ReadAll(r)
		if err != nil {
			break
		}

		if err = json.Unmarshal([]byte(f), &j); err != nil {
			break
		}

		if err = j.Insert(); err != nil {
			break
		}
	}

	if err != nil {
		c.HTML(http.StatusOK, "jobs_upload", gin.H{
			"title":   title,
			"alert":   err.Error(),
			"context": "danger",
			"path":    c.Request.URL.Path,
		})
	} else {
		c.HTML(http.StatusOK, "jobs_upload", gin.H{
			"title":   title,
			"alert":   fmt.Sprintf("Job %s has been successfully uploaded!", j.Name),
			"context": "success",
			"path":    c.Request.URL.Path,
		})
	}
}

func editJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	title := "Edit job"

	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))

	switch c.Request.Method {
	case http.MethodGet:
		if err := j.LoadByGUID(); err != nil {
			otherError(c, map[string]string{
				"alert":    err.Error(),
				"template": "job_edit",
			})
			return
		}
		// TODO: Handle errors.
		agents, _ := loadAgents()
		c.HTML(http.StatusOK, "job_edit", gin.H{
			"title":  title,
			"job":    j,
			"agents": agents,
			"path":   c.Request.URL.Path,
		})
	case http.MethodPost:
		if err := j.LoadByGUID(); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		if err := j.allowConfigChange(); err != nil {
			otherError(c, map[string]string{"alert": err.Error()})
			return
		}
		// Set default values for empty checkboxes.
		j.CrashMode = 0
		j.DirtyMode = 0
		j.DumbMode = 0
		j.PersistCache = 0
		j.BugBucket = 0
		j.ExpertMode = 0
		j.VariableMode = 0
		j.SequentialMode = 0
		j.NoAffinity = 0
		j.SkipCrashes = 0
		j.ShuffleQueue = 0
		j.Autoresume = 0
		if err := c.ShouldBind(&j); err != nil {
			otherError(c, map[string]string{
				"title": title,
				"alert": err.Error(),
			})
			return
		}
		if err := j.Update(); err != nil {
			otherError(c, map[string]string{
				"title": title,
				"alert": err.Error(),
			})
			return
		}
		c.Redirect(http.StatusFound, "/jobs/view")
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}

func viewJobs(c *gin.Context) {
	title := "Jobs"

	jobs, err := loadJobs()
	if err != nil {
		otherError(c, map[string]string{
			"title":    title,
			"alert":    err.Error(),
			"template": "jobs_view",
		})
		return
	}

	c.HTML(http.StatusOK, "jobs_view", gin.H{
		"title": title,
		"jobs":  jobs,
		"path":  c.Request.URL.Path,
	})
}

func downloadJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	t := time.Now()
	date := fmt.Sprintf("%d%02d%02d", t.Year(), t.Month(), t.Day())
	filename := fmt.Sprintf("winaflpet_%s_%s.json", j.Name, date)

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.IndentedJSON(http.StatusOK, j)
}

func alertJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	claims := jwt.ExtractClaims(c)
	user := newUser()
	user.UserName = claims[identityKey].(string)
	user.LoadByUsername()

	m, err := mail.ParseAddress(user.Email)
	if err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	if ok, _ := alert.FindJob(j.GUID); !ok {
		alert.AddJob(*j)

		go alert.Monitor(*j, m)

		c.JSON(http.StatusOK, gin.H{
			"alert":   fmt.Sprintf("Alerts have been enabled for job %s", j.Name),
			"context": "info",
		})
	} else {
		otherError(c, map[string]string{
			"alert": fmt.Sprintf("Alerts are already enabled for job %s", j.Name),
		})
	}
}

func inputJob(c *gin.Context) {
	unlock := lockJob(c.Param("guid"))
	defer unlock()
	j := newJob()
	guid, err := xid.FromString(c.Param("guid"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	j.GUID = guid
	if err = j.LoadByGUID(); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	canUpload := false
	render := func(code int, message, context string) {
		c.HTML(code, "job_input", gin.H{"job": j, "can_upload": canUpload, "title": "Upload input files", "path": c.Request.URL.Path, "alert": message, "context": context})
	}
	if err = j.confirmInputUpload(); err != nil {
		render(http.StatusConflict, err.Error(), "danger")
		return
	}
	canUpload = true
	if c.Request.Method == http.MethodGet {
		render(http.StatusOK, "", "empty")
		return
	}
	if strings.TrimSpace(j.Input) == "" || j.Input == "-" {
		render(http.StatusBadRequest, "Set and save an Input directory before uploading. '-' is a resume marker, not a directory.", "danger")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxInputZIPBytes+(1<<20))
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()
	if err = c.Request.ParseMultipartForm(1 << 20); err != nil {
		render(http.StatusBadRequest, "Invalid upload or ZIP exceeds 256 MiB.", "danger")
		return
	}
	header, err := c.FormFile("input")
	if err != nil {
		render(http.StatusBadRequest, "Select a ZIP file.", "danger")
		return
	}
	if header.Size <= 0 || header.Size > maxInputZIPBytes {
		render(http.StatusBadRequest, "ZIP must be between 1 byte and 256 MiB.", "danger")
		return
	}
	file, err := header.Open()
	if err != nil {
		render(http.StatusBadRequest, err.Error(), "danger")
		return
	}
	defer file.Close()
	a, err := j.GetAgent()
	if err != nil {
		render(http.StatusBadGateway, err.Error(), "danger")
		return
	}
	result, err := sendInputZIP(c.Request.Context(), fmt.Sprintf("http://%s:%d/job/%s/input", a.Host, a.Port, j.GUID), a.Key, j, file, header.Size)
	if err != nil {
		render(http.StatusBadGateway, err.Error(), "danger")
		return
	}
	render(http.StatusOK, fmt.Sprintf("Uploaded %d input files (%d bytes) to %s.", result.Files, result.Bytes, result.Directory), "success")
}
