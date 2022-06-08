package main

import (
	"log"
	"net/http"
	"os"

	sq "github.com/Masterminds/squirrel"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/sgabe/structable"
)

const (
	TB_NAME_USERS   = "users"
	TB_SCHEMA_USERS = `CREATE TABLE users (
		"id" INTEGER PRIMARY KEY AUTOINCREMENT,
		"username" TEXT UNIQUE NOT NULL,
		"password" TEXT NOT NULL,
		"firstname" TEXT NOT NULL,
		"lastname" TEXT NOT NULL,
		"email" TEXT NOT NULL
	);`
)

type User struct {
	structable.Recorder
	ID                      int    `stbl:"id, PRIMARY_KEY, AUTO_INCREMENT"`
	UserName                string `json:"username" form:"username" stbl:"username, UNIQUE"`
	Password                string `json:"password" form:"password" stbl:"password, NOT NULL"`
	NewPassword             string `json:"newPassword" form:"newPassword"`
	NewPasswordConfirmation string `json:"newPasswordConfirmation" form:"newPasswordConfirmation"`
	FirstName               string `json:"firstname" form:"firstname" stbl:"firstname"`
	LastName                string `json:"lastname" form:"lastname" stbl:"lastname"`
	Email                   string `json:"email" form:"email" stbl:"email"`
}

func newUser() *User {
	u := new(User)
	u.Recorder = structable.New(db, DB_FLAVOR).Bind(TB_NAME_USERS, u)
	return u
}

func (u *User) LoadByUsername() error {
	return u.Recorder.LoadWhere("username = ?", u.UserName)
}

func initUser() {
	log.Printf("Creating '%s' user\n", DEFAULT_USER_NAME)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err.Error())
	}

	password, err := generatePassword(hostname)
	if err != nil {
		log.Fatal(err.Error())
	}

	db := getDB()
	if _, err := sq.Insert("users").
		Columns("username", "password", "firstname", "lastname", "email").
		Values(DEFAULT_USER_NAME, password, "", "", "").
		RunWith(db).Exec(); err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("User '%s' created\n", DEFAULT_USER_NAME)
}

func editUser(c *gin.Context) {
	title := "Edit user"

	claims := jwt.ExtractClaims(c)
	user := newUser()
	user.UserName = claims[identityKey].(string)
	user.LoadByUsername()

	switch c.Request.Method {
	case http.MethodGet:
		c.HTML(http.StatusOK, "user_edit", gin.H{
			"title": title,
			"user":  user,
		})
		return
	case http.MethodPost:
		if ok, err := comparePassword(c.PostForm("password"), user.Password); !ok || err != nil {
			c.HTML(http.StatusOK, "user_edit", gin.H{
				"title":   title,
				"alert":   "Password invalid!",
				"user":    user,
				"context": "danger",
			})
			return
		}

		oriPassword := user.Password
		if err := c.ShouldBind(&user); err != nil {
			otherError(c, map[string]string{
				"title":    title,
				"alert":    err.Error(),
				"template": "user_edit",
			})
			return
		}
		user.Password = oriPassword

		if user.NewPassword != "" {
			if user.NewPassword != user.NewPasswordConfirmation {
				c.HTML(http.StatusOK, "user_edit", gin.H{
					"title":   title,
					"alert":   "The password confirmation does not match.",
					"user":    user,
					"context": "danger",
				})
				return
			}
			user.Password, _ = generatePassword(user.NewPassword)
		}

		if err := user.Update(); err != nil {
			c.HTML(http.StatusOK, "user_edit", gin.H{
				"title":   title,
				"alert":   err.Error(),
				"user":    user,
				"context": "danger",
			})
			return
		}

		c.HTML(http.StatusOK, "user_edit", gin.H{
			"title":   title,
			"alert":   "User profile successfully updated.",
			"user":    user,
			"context": "success",
		})
		return
	default:
		c.JSON(http.StatusInternalServerError, gin.H{})
	}
}
