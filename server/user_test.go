package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitUser(t *testing.T) {
	u := newUser()
	u.UserName = DEFAULT_USER_NAME
	err := u.LoadByUsername()

	hostname, _ := os.Hostname()
	match, _ := comparePassword(hostname, u.Password)

	assert.Equal(t, nil, err, "Default user should exist")
	assert.True(t, match, "Default password should match the hostname")
}

func TestLoginUser(t *testing.T) {
	router := setupRouter()
	hostname, _ := os.Hostname()

	data := url.Values{}
	data.Set("username", DEFAULT_USER_NAME)
	data.Set("password", hostname)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/user/login", strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	resp := w.Result()
	location, _ := resp.Location()

	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/jobs/view", location.String())
}
