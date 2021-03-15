package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Masterminds/squirrel"
	"github.com/gin-gonic/gin"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	DEFAULT_LOG = "winaflpet.log"

	DEFAULT_SERVER_HOST = "127.0.0.1"
	DEFAULT_SERVER_PORT = 4141

	DEFAULT_DATA_DIR      = "data"
	DEFAULT_DATABASE_TYPE = "sqlite3"
	DEFAULT_DATABASE_NAME = "database.db"

	DEFAULT_USER_NAME = "admin"
)

var (
	BuildVer string
	BuildRev string
	db       squirrel.DBProxyBeginner
)

func setDefaults() {
	viper.SetDefault("data.dir", DEFAULT_DATA_DIR)
	viper.BindEnv("data.dir", "WINAFLPET_DATA")

	viper.SetDefault("server.host", DEFAULT_SERVER_HOST)
	viper.BindEnv("server.host", "WINAFLPET_HOST")

	viper.SetDefault("server.port", DEFAULT_SERVER_PORT)
	viper.BindEnv("server.port", "WINAFLPET_PORT")

	viper.SetDefault("log", DEFAULT_LOG)
	viper.BindEnv("log", "WINAFLPET_LOG")
}

func main() {
	var (
		host    string
		port    int
		log     string
		config  string
		version bool
	)

	setDefaults()

	flag.StringVarP(&host, "host", "h", DEFAULT_SERVER_HOST, "Host to bind to")
	viper.BindPFlag("server.host", flag.Lookup("host"))

	flag.IntVarP(&port, "port", "p", DEFAULT_SERVER_PORT, "Port to bind to")
	viper.BindPFlag("server.port", flag.Lookup("port"))

	flag.StringVarP(&log, "log", "l", DEFAULT_LOG, "Log filename")
	viper.BindPFlag("log", flag.Lookup("log"))

	flag.StringVarP(&config, "config", "c", "", "Configuration filename")
	flag.BoolVarP(&version, "version", "v", false, "Output the current version of the server")

	flag.Parse()

	if version {
		fmt.Printf("WinAFL Pet Server v%s (rev %s)\n", BuildVer, BuildRev)
		os.Exit(0)
	}

	if config != "" {
		viper.SetConfigFile(config)
	} else {
		viper.SetConfigName("winaflpet")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(viper.GetString("data.dir"))
	}

	if err := viper.ReadInConfig(); err != nil {
		if config != "" {
			fmt.Println(err)
		}
	}

	db = getDB()

	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)

	f, _ := os.Create(filepath.Join(viper.GetString("data.dir"), viper.GetString("log")))
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

	r := setupRouter()
	addr := fmt.Sprintf("%s:%d", viper.GetString("server.host"), viper.GetInt("server.port"))
	if err := r.Run(addr); err != nil {
		fmt.Println(err)
	}
}
