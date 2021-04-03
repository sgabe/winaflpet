package main

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig"
	jwt "github.com/appleboy/gin-jwt/v2"
	helmet "github.com/danielkov/gin-helmet"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
)

func customHTMLRender() multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	r.AddFromString("50x", "Ooops...")
	r.AddFromFiles("user_login", "templates/user_login.html")

	funcs := template.FuncMap{
		"seq":            seq,
		"hasStatus":      hasStatus,
		"getVersion":     getVersion,
		"totalPages":     totalPages,
		"formatNumber":   formatNumber,
		"formatDuration": formatDuration,
	}

	for k, v := range sprig.FuncMap() {
		funcs[k] = v
	}

	layouts, err := filepath.Glob("templates/layouts/*.html")
	if err != nil {
		panic(err.Error())
	}

	includes, err := filepath.Glob("templates/includes/*.html")
	if err != nil {
		panic(err.Error())
	}

	for _, include := range includes {
		fileName := filepath.Base(include)
		name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)
		files := append(layoutCopy, include)

		r.AddFromFilesFuncs(name, funcs, files...)
	}

	return r
}

func home(c *gin.Context) {
	c.HTML(http.StatusOK, "home", gin.H{})
}

func notFound(c *gin.Context) {
	redirectURL := "/user/login"

	claims := jwt.ExtractClaims(c)
	if len(claims) != 0 {
		redirectURL = "/jobs/view"
	}

	redirectCode := fmt.Sprintf("<head><meta http-equiv='refresh' content='0; URL=%s'></head>", redirectURL)

	c.Data(http.StatusNotFound, "text/html", []byte(redirectCode))
}

func notImplemented(c *gin.Context) {
	c.HTML(http.StatusNotImplemented, "50x", gin.H{})
}

func otherError(c *gin.Context, p map[string]string) {
	alert := ""
	if p["alert"] != "" {
		alert = p["alert"]
	}

	template := ""
	if p["template"] != "" {
		template = p["template"]
	}

	context := "danger"
	if p["context"] != "" {
		context = p["context"]
	}

	title := ""
	if p["title"] != "" {
		title = p["title"]
	}

	if template == "" {
		c.JSON(http.StatusOK, gin.H{
			"alert":   alert,
			"context": context,
		})
		return
	}

	c.HTML(http.StatusOK, template, gin.H{
		"title":   title,
		"alert":   alert,
		"context": context,
	})
}

func setupRouter() *gin.Engine {
	e := gin.Default()
	e.Use(helmet.Default())
	e.HTMLRender = customHTMLRender()

	e.Static("/static", "./public/static")
	e.Static("/plots", "./public/plots")
	e.StaticFile("/favicon.ico", "./public/static/svg/carrot-solid.svg")

	e.POST("/ping", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain", []byte("pong"))
	})

	auth, err := Authentication()
	if err != nil {
		fmt.Println("JWT Error:" + err.Error())
	}

	errInit := auth.MiddlewareInit()
	if errInit != nil {
		fmt.Println("auth.MiddlewareInit() Error:" + errInit.Error())
	}

	e.NoRoute(auth.MiddlewareFunc(), notFound)
	e.NoMethod(auth.MiddlewareFunc(), notImplemented)

	u := e.Group("/user")
	u.POST("/login", auth.LoginHandler)
	u.GET("/login", func(c *gin.Context) { c.HTML(http.StatusOK, "user_login", gin.H{}) })

	r := e.Group("/")
	r.Use(auth.MiddlewareFunc())
	{
		r.GET("/", home)

		r.GET("/user/edit", editUser)
		r.POST("/user/edit", editUser)
		r.GET("/user/logout", auth.LogoutHandler)
		r.POST("/user/refresh", auth.RefreshHandler)

		r.GET("/jobs/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "create":
				createJobs(c)
			case "view":
				viewJobs(c)
			default:
				notFound(c)
			}
		})

		r.PUT("/jobs/create", createJobs)

		r.GET("/job/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "view":
				viewJob(c)
			case "edit":
				editJob(c)
			case "plot":
				plotJob(c)
			default:
				notFound(c)
			}
		})

		r.POST("/job/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "start":
				startJob(c)
			case "stop":
				stopJob(c)
			case "edit":
				editJob(c)
			case "check":
				checkJob(c)
			case "collect":
				collectJob(c)
			case "plot":
				plotJob(c)
			default:
				notImplemented(c)
			}
		})

		r.DELETE("/job/:guid", deleteJob)

		r.GET("/crashes/view", viewCrashes)
		r.DELETE("/crashes", deleteCrashes)

		r.GET("/crash/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "edit":
				editCrash(c)
			default:
				notFound(c)
			}
		})

		r.POST("/crash/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "download":
				downloadCrash(c)
			case "edit":
				editCrash(c)
			case "verify":
				verifyCrash(c)
			default:
				notImplemented(c)
			}
		})

		r.DELETE("/crash/:guid", deleteCrash)

		r.GET("/agents/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "create":
				createAgents(c)
			case "view":
				viewAgents(c)
			default:
				notFound(c)
			}
		})

		r.PUT("/agents/create", createAgents)
		r.DELETE("/agents", deleteAgents)

		r.GET("/agent/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "edit":
				editAgent(c)
			default:
				notFound(c)
			}
		})

		r.POST("/agent/:guid/:action", func(c *gin.Context) {
			switch c.Param("action") {
			case "edit":
				editAgent(c)
			case "check":
				checkAgent(c)
			default:
				notImplemented(c)
			}
		})

		r.DELETE("/agent/:guid", deleteAgent)
	}

	return e
}
