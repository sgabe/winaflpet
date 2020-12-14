package main

import (
	"io/ioutil"

	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthRoute(t *testing.T) {
	router := setupRouter()

	data := url.Values{}
	data.Set("username", "wrongusername")
	data.Set("password", "wrongpassword")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/user/login", strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, 401, resp.StatusCode)
	assert.Contains(t, string(body), "Invalid username or password.")
}
