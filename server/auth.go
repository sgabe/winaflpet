package main

import (
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

const (
	identityKey  = "identity"
	redirectCode = "<head><meta http-equiv='refresh' content='0; URL=/user/login'></head>"
)

type creds struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

func Authentication() (*jwt.GinJWTMiddleware, error) {
	middleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "WinAFL Pet",
		Key:         generateSecretKey(),
		Timeout:     time.Hour,
		MaxRefresh:  time.Hour,
		IdentityKey: identityKey,
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.SetCookie("token", token, 3600, "/", "", true, true)
			c.Redirect(http.StatusFound, "/jobs/view")
		},
		LogoutResponse: func(c *gin.Context, code int) {
			c.Data(http.StatusOK, "text/html", []byte(redirectCode))
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					identityKey: v.UserName,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			user := newUser()
			user.UserName = claims[identityKey].(string)
			user.LoadByUsername()
			return user
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var creds creds
			if err := c.ShouldBind(&creds); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			user := newUser()
			user.UserName = creds.Username
			if err := user.LoadByUsername(); err == nil {
				if ok, err := comparePassword(creds.Password, user.Password); ok && err == nil {
					return user, nil
				}
			}
			return nil, jwt.ErrFailedAuthentication
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			if message == "incorrect Username or Password" {
				c.HTML(http.StatusUnauthorized, "user_login", gin.H{
					"alert":   "Invalid username or password.",
					"context": "danger",
				})
				return
			}
			c.Data(http.StatusUnauthorized, "text/html", []byte(redirectCode))
		},
		SendCookie:     true,
		SecureCookie:   true,
		CookieHTTPOnly: true,
		CookieName:     "token",
		TokenLookup:    "header: Authorization, cookie: token",
	})

	return middleware, err
}
