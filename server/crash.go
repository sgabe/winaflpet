package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
	"github.com/rs/xid"
	"github.com/sgabe/structable"
)

const (
	TB_NAME_CRASHES   = "crashes"
	TB_SCHEMA_CRASHES = `
		CREATE TABLE crashes (
			"id" INTEGER PRIMARY KEY AUTOINCREMENT,
			"jid" INTEGER,
			"guid" TEXT UNIQUE,
			"label" TEXT NOT NULL,
			"fuzzerid" TEXT,
			"bugid" TEXT,
			"mod" TEXT,
			"func" TEXT,
			"desc" TEXT,
			"imp" TEXT,
			"args" TEXT,
			"verified" INTEGER,
			FOREIGN KEY (jid) REFERENCES jobs(id)
		);`
)

type Crash struct {
	structable.Recorder
	ID          int    `stbl:"id, PRIMARY_KEY, AUTO_INCREMENT"`
	JobID       int    `json:"jid" stbl:"jid"`
	GUID        xid.ID `json:"guid" stbl:"guid"`
	JobGUID     xid.ID `json:"jguid"`
	Label       string `json:"label" form:"label" stbl:"label"`
	FuzzerID    string `json:"fuzzerid" form:"fuzzerid" stbl:"fuzzerid"`
	BugID       string `json:"bugid" form:"bugid" stbl:"bugid"`
	Module      string `json:"mod" form:"mod" stbl:"mod"`
	Function    string `json:"func" form:"func" stbl:"func"`
	Description string `json:"desc" form:"desc" stbl:"desc"`
	Impact      string `json:"imp" form:"imp" stbl:"imp"`
	Args        string `json:"args" form:"args" stbl:"args"`
	Verified    bool   `stbl:"verified" form:"verified"`
}

func newCrash() *Crash {
	c := new(Crash)
	c.GUID = xid.New()
	c.Recorder = structable.New(db, DB_FLAVOR).Bind(TB_NAME_CRASHES, c)
	return c
}

func (c *Crash) LoadByGUID() error {
	return c.Recorder.LoadWhere("guid = ?", c.GUID)
}

func (c *Crash) LoadByJobIDArgs() error {
	return c.Recorder.LoadWhere("jid = ? and args = ?", c.JobID, c.Args)
}

func (c *Crash) GetJob() (*Job, error) {
	j := newJob()
	j.ID = c.JobID
	if err := j.Load(); err != nil {
		return j, err
	}
	return j, nil
}

func (c *Crash) GetRisk() string {
	risk := "none"

	re := regexp.MustCompile(`\w{2,3}(R|W|E)\W?`)
	matches := re.FindStringSubmatch(c.BugID)
	if matches == nil {
		return risk
	}

	switch matches[1] {
	case "R":
		risk = "low"
	case "W":
		risk = "medium"
	case "E":
		risk = "high"
	}

	return risk
}

func loadCrashes(page uint64) ([]*Crash, error) {
	c := &Crash{}
	sc := structable.New(db, DB_FLAVOR).Bind(TB_NAME_CRASHES, c)

	fn := func(d structable.Describer, q sq.SelectBuilder) (sq.SelectBuilder, error) {
		return q.OrderBy("id DESC").Limit(99).Offset(page * 99), nil
	}

	items, err := listWhere(sc, fn)
	if err != nil {
		return []*Crash{}, err
	}

	// Because we get back a []Recorder, we need to get the original data
	// back out. We have to manually convert it back to its real type.
	crashes := make([]*Crash, len(items))
	for i, item := range items {
		crashes[i] = item.Interface().(*Crash)
	}

	return crashes, err
}

func viewCrashes(c *gin.Context) {
	title := "Crashes"
	currentPage := 0

	p, err := strconv.Atoi(c.DefaultQuery("p", "1"))
	if err == nil && p > 0 && p < (totalPages()+1) {
		currentPage = p - 1
	}

	crashes, err := loadCrashes(uint64(currentPage))
	if err != nil {
		otherError(c, map[string]string{
			"title":    title,
			"alert":    err.Error(),
			"template": "crashes_view",
		})
		return
	}

	c.HTML(http.StatusOK, "crashes_view", gin.H{
		"title":       title,
		"crashes":     crashes,
		"currentPage": currentPage,
		"path":        c.Request.URL.Path,
	})
}

func deleteCrashes(c *gin.Context) {
	b := sq.Delete("").From(TB_NAME_CRASHES).RunWith(db)

	_, err := b.Exec()
	if err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   "All crash records have been successfully deleted!",
		"context": "success",
	})
}

func verifyCrash(c *gin.Context) {
	crash := newCrash()
	crash.GUID, _ = xid.FromString(c.Param("guid"))
	if err := crash.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	j := newJob()
	j.ID = crash.JobID
	j.Load()

	a, _ := j.GetAgent()
	crash.JobGUID = j.GUID

	request := gorequest.New()
	request.Debug = false

	targetURL := fmt.Sprintf("http://%s:%d/crash/%s/verify", a.Host, a.Port, crash.GUID)
	_, bodyBytes, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).Send(crash).EndStruct(&crash)
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

	crash.Verified = true

	if err := crash.Update(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   fmt.Sprintf("A(n) %s bug was detected in %s at %s!", crash.BugID, crash.Module, crash.Function),
		"context": "success",
	})
}

func deleteCrash(c *gin.Context) {
	crash := newCrash()
	crash.GUID, _ = xid.FromString(c.Param("guid"))
	if err := crash.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if err := crash.Delete(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   "Crash successfully deleted!",
		"context": "success",
	})
}

func editCrash(c *gin.Context) {
	title := "Edit crash"

	crash := newCrash()
	crash.GUID, _ = xid.FromString(c.Param("guid"))

	switch c.Request.Method {
	case http.MethodGet:
		if err := crash.LoadByGUID(); err != nil {
			otherError(c, map[string]string{
				"alert":    err.Error(),
				"template": "crash_edit",
			})
			return
		}
		c.HTML(http.StatusOK, "crash_edit", gin.H{
			"title": title,
			"crash": crash,
			"path":  c.Request.URL.Path,
		})
	case http.MethodPost:
		if err := crash.LoadByGUID(); err != nil {
			otherError(c, map[string]string{"alert": err.Error()})
			return
		}
		if err := c.ShouldBind(&crash); err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "job_edit",
			})
			return
		}
		if err := crash.Update(); err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "crash_edit",
			})
			return
		}
		c.Redirect(http.StatusFound, "/crashes/view")
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}

func downloadCrash(c *gin.Context) {
	crash := newCrash()
	crash.GUID, _ = xid.FromString(c.Param("guid"))
	if err := crash.LoadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	j := newJob()
	j.ID = crash.JobID
	j.Load()

	a, _ := j.GetAgent()
	crash.JobGUID = j.GUID

	request := gorequest.New()
	request.Debug = false

	targetURL := fmt.Sprintf("http://%s:%d/crash/%s/download", a.Host, a.Port, crash.GUID)
	resp, bodyBytes, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).Send(crash).EndBytes()
	if errs != nil {
		otherError(c, map[string]string{"alert": errs[0].Error()})
		return
	}

	if resp.StatusCode != http.StatusOK {
		resp := APIResponse{}
		if err := json.Unmarshal(bodyBytes, &resp); err != nil {
			otherError(c, map[string]string{
				"alert":   err.Error(),
				"context": "danger",
			})
			return
		}

		if len(resp.Err) > 0 {
			otherError(c, map[string]string{
				"alert":   resp.Err,
				"context": "danger",
			})
			return
		}
	}

	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(strings.Replace(crash.Args, "\\", "/", -1)))

	c.Data(http.StatusOK, "application/octet-stream", bodyBytes)
}
