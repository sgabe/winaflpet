// +build windows

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
	"github.com/pjebs/restgate"
)

var (
	isFirstRun bool = true
	project    Project
)

type Agent struct {
	Server http.Server
	exit   chan struct{}
}

func (a *Agent) Start(s service.Service) error {
	if service.Interactive() {
		logger.Info("Running in terminal.")
	} else {
		logger.Info("Running under service manager.")
	}
	a.exit = make(chan struct{})

	go a.Run()
	return nil
}

func (a *Agent) Run() {
	if isFirstRun {
		isFirstRun = false

		logger.Info("Started HTTP Server.")

		r := gin.Default()
		key, err := getKey()
		if err != nil {
			logger.Error(err.Error())
		}

		rg := restgate.New("X-Auth-Key", "", restgate.Static, restgate.Config{
			Key:                []string{key},
			Debug:              true,
			HTTPSProtectionOff: true,
		})

		rgAdapter := func(c *gin.Context) {
			nextCalled := false
			nextAdapter := func(http.ResponseWriter, *http.Request) {
				nextCalled = true
				c.Next()
			}
			rg.ServeHTTP(c.Writer, c.Request, nextAdapter)
			if !nextCalled {
				c.AbortWithStatus(401)
			}
		}

		r.Use(rgAdapter)

		r.POST("/ping", func(c *gin.Context) {
			c.Header("X-WinAFLPet-Ver", fmt.Sprintf("%s (rev %s)", BuildVer, BuildRev))
			c.Data(http.StatusOK, "text/plain", []byte("pong"))
		})

		r.POST("/job/:guid/:action", func(c *gin.Context) {
			switch action := c.Param("action"); action {
			case "start":
				startJob(c)
			case "stop":
				stopJob(c)
			case "view":
				viewJob(c)
			case "check":
				checkJob(c)
			case "collect":
				collectJob(c)
			case "plot":
				plotJob(c)
			default:
				c.JSON(http.StatusNotImplemented, gin.H{})
			}
		})

		r.POST("/crash/:guid/:action", func(c *gin.Context) {
			switch action := c.Param("action"); action {
			case "verify":
				verifyCrash(c)
			case "download":
				downloadCrash(c)
			default:
				c.JSON(http.StatusNotImplemented, gin.H{})
			}
		})

		a.Server = http.Server{
			Addr:    ":8080",
			Handler: r,
		}

		go func() {
			if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Errorf("Server was unable to start: %s", err)
			}
		}()
	}
}
func (a *Agent) Stop(s service.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Server.Shutdown(ctx); err != nil {
		logger.Errorf("Server was forced to shutdown: %s", err)
	}

	return nil
}
