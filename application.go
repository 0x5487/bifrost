package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jasonsoft/napnap"
)

type Application struct {
	Hostname string
}

func newApplication() *Application {
	name, err := os.Hostname()
	panicIf(err)

	return &Application{
		Hostname: name,
	}
}

func notFound(c *napnap.Context, next napnap.HandlerFunc) {
	c.SetStatus(404)
}

func auth(c *napnap.Context, next napnap.HandlerFunc) {
	if len(_config.AdminTokens) == 0 {
		next(c)
		return
	} else {
		key := c.RequestHeader("Authorization")
		if len(key) == 0 {
			c.SetStatus(401)
			return
		}

		var isFound bool
		for _, token := range _config.AdminTokens {
			if token == key {
				isFound = true
				break
			}
		}

		if isFound {
			next(c)
		} else {
			c.SetStatus(401)
		}
	}
}

type AppError struct {
	RequestID string    `json:"-" bson:"_id"`
	Hostname  string    `json:"-" bson:"hostname"`
	ErrorCode string    `json:"error_code" bson:"-"`
	Message   string    `json:"message" bson:"message"`
	CreatedAt time.Time `json:"-" bson:"created_at"`
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s - %s", e.ErrorCode, e.Message)
}

type ApiCount struct {
	Count int `json:"count"`
}
