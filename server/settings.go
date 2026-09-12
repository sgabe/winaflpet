package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

const settingsSchema = `CREATE TABLE IF NOT EXISTS settings (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 smtp_host TEXT NOT NULL, smtp_port INTEGER NOT NULL,
 smtp_username TEXT NOT NULL, smtp_password TEXT NOT NULL,
 alert_interval INTEGER NOT NULL
)`

type Settings struct {
	Host     string `form:"smtp_host"`
	Port     int    `form:"smtp_port"`
	Username string `form:"smtp_username"`
	Password string `form:"smtp_password"`
	Interval int    `form:"alert_interval"`
}

var settingsMu sync.Mutex

func loadSettings() (Settings, error) {
	s := Settings{
		Host: viper.GetString("smtp.host"), Port: viper.GetInt("smtp.port"),
		Username: viper.GetString("smtp.username"), Password: viper.GetString("smtp.password"),
		Interval: viper.GetInt("alert.interval"),
	}
	if s.Port == 0 {
		s.Port = 587
	}
	if s.Interval < 1 || s.Interval > 10080 {
		s.Interval = 10
	}
	var saved Settings
	err := db.QueryRow(`SELECT smtp_host, smtp_port, smtp_username, smtp_password, alert_interval FROM settings WHERE id = 1`).Scan(&saved.Host, &saved.Port, &saved.Username, &saved.Password, &saved.Interval)
	if errors.Is(err, sql.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return saved, nil
}

func (s Settings) validate() error {
	if s.Host == "" || strings.ContainsAny(s.Host, " /\\\t\r\n") || (strings.Contains(s.Host, ":") && net.ParseIP(s.Host) == nil) {
		return fmt.Errorf("Enter an SMTP hostname or IP address without a port or URL scheme.")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535.")
	}
	if s.Username == "" || strings.ContainsAny(s.Username, "\r\n") {
		return fmt.Errorf("Enter an SMTP username. It is also used as the sender address.")
	}
	if s.Interval < 1 || s.Interval > 10080 {
		return fmt.Errorf("Alert interval must be between 1 and 10080 minutes.")
	}
	return nil
}

func saveSettings(s Settings, clearPassword bool) error {
	if err := s.validate(); err != nil {
		return err
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if clearPassword {
		s.Password = ""
	} else if s.Password == "" {
		current, err := loadSettings()
		if err != nil {
			return err
		}
		s.Password = current.Password
	}
	_, err := db.Exec(`INSERT INTO settings (id, smtp_host, smtp_port, smtp_username, smtp_password, alert_interval)
 VALUES (1, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET
 smtp_host=excluded.smtp_host, smtp_port=excluded.smtp_port,
 smtp_username=excluded.smtp_username, smtp_password=excluded.smtp_password,
 alert_interval=excluded.alert_interval`, s.Host, s.Port, s.Username, s.Password, s.Interval)
	return err
}

func editSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := loadSettings()
	render := func(code int, message, context string) {
		// Never put the stored or submitted password into the rendered page.
		settings.Password = ""
		c.HTML(code, "settings_edit", gin.H{"settings": settings, "title": "Settings", "path": c.Request.URL.Path, "alert": message, "context": context})
	}
	if err != nil {
		render(http.StatusInternalServerError, "Unable to load settings.", "danger")
		return
	}
	if c.Request.Method == http.MethodPost {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
		var submitted Settings
		if err = c.ShouldBind(&submitted); err != nil {
			render(http.StatusBadRequest, "Enter valid numeric values for the port and interval.", "danger")
			return
		}
		submitted.Host = strings.TrimSpace(submitted.Host)
		submitted.Username = strings.TrimSpace(submitted.Username)
		settings = submitted
		if err = settings.validate(); err != nil {
			render(http.StatusBadRequest, err.Error(), "danger")
			return
		}
		if err = saveSettings(settings, c.PostForm("clear_password") == "on"); err != nil {
			render(http.StatusInternalServerError, "Unable to save settings.", "danger")
			return
		}
		c.Redirect(http.StatusSeeOther, "/settings?saved=1")
		return
	}
	if c.Query("saved") == "1" {
		render(http.StatusOK, "Settings saved. Active alerts use the changes at their next scheduled check.", "success")
		return
	}
	render(http.StatusOK, "", "empty")
}
