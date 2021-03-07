package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/Masterminds/structable"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
	"github.com/rs/xid"
)

const (
	TB_NAME_AGENTS   = "agents"
	TB_SCHEMA_AGENTS = `CREATE TABLE agents (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"guid" TEXT NOT NULL,
		"name" TEXT,
		"desc" TEXT,
		"host" TEXT NOT NULL,
		"port" INTEGER NOT NULL,
		"key" TEXT NOT NULL,
		"ver" TEXT,
		"status" INTEGER
	  );`
)

type Agent struct {
	structable.Recorder
	ID          int    `stbl:"id, PRIMARY_KEY, AUTO_INCREMENT"`
	GUID        xid.ID `json:"guid" stbl:"guid"`
	Name        string `json:"name" form:"name" stbl:"name"`
	Description string `json:"desc" form:"desc" stbl:"desc"`
	Host        string `json:"host" form:"host" stbl:"host"`
	Port        int    `json:"port" form:"port" stbl:"port"`
	Key         string `json:"key" form:"key" stbl:"key"`
	Version     string `json:"ver" form:"ver" stbl:"ver"`
	Status      int    `json:"status" form:"status" stbl:"status"`
}

func newAgent() *Agent {
	a := new(Agent)
	a.GUID = xid.New()
	a.Status = 1
	a.Recorder = structable.New(db, DB_FLAVOR).Bind(TB_NAME_AGENTS, a)
	return a
}

func (a *Agent) loadByGUID() error {
	return a.Recorder.LoadWhere("guid = ?", a.GUID)
}

func loadAgents() ([]*Agent, error) {
	a := &Agent{}
	sa := structable.New(db, DB_FLAVOR).Bind(TB_NAME_AGENTS, a)

	fn := func(d structable.Describer, q squirrel.SelectBuilder) (squirrel.SelectBuilder, error) {
		return q.Limit(100), nil
	}

	items, err := listWhere(sa, fn)
	if err != nil {
		return []*Agent{}, err
	}

	// Because we get back a []Recorder, we need to get the original data
	// back out. We have to manually convert it back to its real type.
	agents := make([]*Agent, len(items))
	for i, item := range items {
		agents[i] = item.Interface().(*Agent)
	}

	return agents, err
}

func createAgents(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		c.HTML(http.StatusOK, "agents_create", gin.H{
			"title": "Create agent",
		})
		return
	case http.MethodPut:
		a := newAgent()
		if err := c.ShouldBind(&a); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		if err := a.Insert(); err != nil {
			otherError(c, map[string]string{
				"alert": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"alert":   fmt.Sprintf("Agent %s has been successfully created!", a.Name),
			"context": "success",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}

func viewAgents(c *gin.Context) {
	title := "Agents"

	agents, err := loadAgents()
	if err != nil {
		otherError(c, map[string]string{
			"title":    title,
			"alert":    err.Error(),
			"template": "agents_view",
		})
		return
	}

	c.HTML(http.StatusOK, "agents_view", gin.H{
		"title":  title,
		"agents": agents,
	})
}

func deleteAgents(c *gin.Context) {
	agents := squirrel.Select("id").From(TB_NAME_AGENTS)
	rows, err := agents.RunWith(db).Query()
	if err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
	}

	defer rows.Close()

	for rows.Next() {
		a := newAgent()
		if err := rows.Scan(&a.ID); err != nil {
			otherError(c, map[string]string{"alert": err.Error()})
		}
		if err := a.Load(); err != nil {
			otherError(c, map[string]string{"alert": err.Error()})
		}
		a.Delete()
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   "All agents have been successfully deleted!",
		"context": "success",
	})
}

func checkAgent(c *gin.Context) {
	a := newAgent()
	a.GUID, _ = xid.FromString(c.Param("guid"))
	if err := a.loadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	request := gorequest.New().Timeout(1000 * time.Millisecond)
	request.Debug = false

	targetURL := fmt.Sprintf("http://%s:%d/ping", a.Host, a.Port)
	resp, body, errs := request.Post(targetURL).Set("X-Auth-Key", a.Key).End()
	if errs != nil {
		otherError(c, map[string]string{"alert": errs[0].Error()})
		return
	}

	if body != "pong" {
		otherError(c, map[string]string{"alert": body})
		return
	}

	agentVersion := resp.Header.Get("X-WinAFLPet-Ver")
	if agentVersion != "" {
		a.Version = agentVersion
		a.Update()
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   fmt.Sprintf("Agent %s is up and running!", a.Name),
		"context": "success",
	})
}

func deleteAgent(c *gin.Context) {
	a := newAgent()
	a.GUID, _ = xid.FromString(c.Param("guid"))
	if err := a.loadByGUID(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	if err := a.Delete(); err != nil {
		otherError(c, map[string]string{"alert": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alert":   "Agent has been successfully deleted!",
		"context": "success",
	})
}

func editAgent(c *gin.Context) {
	title := "Edit agent"

	a := newAgent()
	a.GUID, _ = xid.FromString(c.Param("guid"))

	switch c.Request.Method {
	case http.MethodGet:
		if err := a.loadByGUID(); err != nil {
			otherError(c, map[string]string{
				"alert":    err.Error(),
				"template": "agent_edit",
			})
			return
		}
		c.HTML(http.StatusOK, "agent_edit", gin.H{
			"title": title,
			"agent": a,
		})
	case http.MethodPost:
		if err := a.loadByGUID(); err != nil {
			otherError(c, map[string]string{"alert": err.Error()})
			return
		}
		if err := c.ShouldBind(&a); err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "agent_edit",
			})
			return
		}
		a.Status, _ = strconv.Atoi(c.DefaultPostForm("status", "0"))
		if err := a.Update(); err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "agent_edit",
			})
			return
		}
		c.Redirect(http.StatusFound, "/agents/view")
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}
