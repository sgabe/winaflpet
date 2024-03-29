package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
		"memory_limit" TEXT,
		"persist_cache" INTEGER,
		"dirty_mode" INTEGER,
		"dumb_mode" INTEGER,
		"crash_mode" INTEGER,
		"expert_mode" INTEGER,
		"skip_crashes" INTEGER,
		"autoresume" INTEGER,
		"shuffle_queue" INTEGER,
		"no_affinity" INTEGER,
		"status" INTEGER,
		FOREIGN KEY (aid) REFERENCES agents(id)
	  );`
)

type Job struct {
	structable.Recorder
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
	MemoryLimit    string `json:"memory_limit" form:"memory_limit" stbl:"memory_limit"`
	PersistCache   int    `json:"persist_cache" form:"persist_cache" stbl:"persist_cache"`
	DirtyMode      int    `json:"dirty_mode" form:"dirty_mode" stbl:"dirty_mode"`
	DumbMode       int    `json:"dumb_mode" form:"dumb_mode" stbl:"dumb_mode"`
	CrashMode      int    `json:"crash_mode" form:"crash_mode" stbl:"crash_mode"`
	ExpertMode     int    `json:"expert_mode" form:"expert_mode" stbl:"expert_mode"`
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
	j.InstMode = "Dynamic" // The only supported instrumentation mode.
	j.CrashMode = 0
	j.DirtyMode = 0
	j.DumbMode = 0
	j.PersistCache = 0
	j.ExpertMode = 0
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

func startJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	// TODO: Handle errors.
	a, _ := j.GetAgent()

	request := gorequest.New().Timeout(300 * time.Second)
	request.Debug = false

	fID, err := strconv.Atoi(c.DefaultQuery("fid", "1"))
	if err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	targetURL := fmt.Sprintf("http://%s:%d/job/%s/start", a.Host, a.Port, j.GUID)
	_, bodyBytes, errs := request.Post(targetURL).Query(fmt.Sprintf("fid=%d", fID)).Set("X-Auth-Key", a.Key).Send(j).EndBytes()
	if errs != nil {
		otherError(c, map[string]string{"alert": errs[0].Error()})
		return
	}

	resp := APIResponse{}
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if len(resp.Err) > 0 {
		if strings.Contains(resp.Err, "At-risk data found") {
			j.Input = "-"
			j.Update()
			j.Cleanup(fID)
			resp.Err = "At-risk data found, try to start again to resume an aborted job."
		}
		otherError(c, map[string]string{"alert": resp.Err})
		return
	}

	j.Status = setStatus(j.Status, statusMap[fID])
	j.Update()

	c.JSON(http.StatusOK, gin.H{
		"alert":   resp.Msg,
		"context": "success",
	})
}

func stopJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	j.Status = 0
	j.Update()

	// TODO: Handle errors.
	a, _ := j.GetAgent()

	request := gorequest.New()
	request.Debug = false

	targetURL := fmt.Sprintf("http://%s:%d/job/%s/stop", a.Host, a.Port, j.GUID)
	_, bodyBytes, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).EndBytes()
	if errs != nil {
		otherError(c, map[string]string{"alert": errs[0].Error()})
		return
	}

	resp := APIResponse{}
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if len(resp.Err) > 0 {
		otherError(c, map[string]string{"alert": resp.Err})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   resp.Msg,
		"context": "success",
	})
}

func deleteJob(c *gin.Context) {
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if err := j.Delete(); err != nil {
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

	var statsTemp []Stat
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
	for _, stat := range statsTemp {
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

	c.HTML(http.StatusOK, "job_view", gin.H{
		"title": title,
		"stats": stats,
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
	j := newJob()
	j.GUID, _ = xid.FromString(c.Param("guid"))
	if err := j.LoadByGUID(); err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	fID, err := strconv.Atoi(c.DefaultQuery("fid", "0"))
	if err != nil {
		otherError(c, map[string]string{
			"alert": err.Error(),
		})
		return
	}

	processIDs, _ := j.GetProcessIDs(fID)
	// TODO: Handle errors.
	a, _ := j.GetAgent()

	request := gorequest.New()
	request.Debug = false

	resp := APIResponse{}
	targetURL := fmt.Sprintf("http://%s:%d/job/%s/check", a.Host, a.Port, j.GUID)
	_, _, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).Send(processIDs).EndStruct(&resp)
	if errs != nil {
		otherError(c, map[string]string{
			"alert": errs[0].Error(),
		})
		return
	}

	if len(resp.Err) > 0 {
		otherError(c, map[string]string{
			"alert": resp.Err,
		})
		return
	}

	if resp.PID != 0 {
		s := newStat()
		s.JobID = j.ID
		s.FuzzerProcessID = resp.PID
		s.LoadJobIDProcessID()

		j.Status = clearStatus(j.Status, statusMap[s.GetFID()])
		j.Update()

		otherError(c, map[string]string{
			"alert": resp.Msg,
		})
		return
	}

	for _, processID := range processIDs {
		s := newStat()
		s.JobID = j.ID
		s.FuzzerProcessID = processID
		s.LoadJobIDProcessID()

		j.Status = setStatus(j.Status, statusMap[s.GetFID()])
		j.Update()
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   resp.Msg,
		"context": "info",
	})
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

		f, err = ioutil.ReadAll(r)
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
		})
	} else {
		c.HTML(http.StatusOK, "jobs_upload", gin.H{
			"title":   title,
			"alert":   fmt.Sprintf("Job %s has been successfully uploaded!", j.Name),
			"context": "success",
		})
	}
}

func editJob(c *gin.Context) {
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
		})
	case http.MethodPost:
		if err := j.LoadByGUID(); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		// Set default values for empty checkboxes.
		j.CrashMode = 0
		j.DirtyMode = 0
		j.DumbMode = 0
		j.PersistCache = 0
		j.ExpertMode = 0
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
