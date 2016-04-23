package main

import (
	"fmt"

	"github.com/jasonsoft/napnap"
)

func notFound(c *napnap.Context, next napnap.HandlerFunc) {
	c.Status(404)
}

type AppError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s - %s", e.ErrorCode, e.Message)
}

type ApiCount struct {
	Count int `json:"count"`
}

type ApplicationMiddleware struct {
}

func newApplicationMiddleware() ApplicationMiddleware {
	return ApplicationMiddleware{}
}

func (m ApplicationMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	defer func() {
		if err := recover(); err != nil {
			var appError AppError
			e, ok := err.(AppError)
			if ok {
				appError = e
				if appError.ErrorCode == "NOT_FOUND" {
					c.JSON(404, appError)
					return
				}
				c.JSON(400, appError)
				return
			}

			// unknown error
			_logger.debugf("unknown error: %v", err)
			appError = AppError{
				ErrorCode: "UNKNOWN_ERROR",
				Message:   "An unknown error has occurred.",
			}
			c.JSON(500, appError)
		}
	}()
	next(c)
}

func auth(c *napnap.Context, next napnap.HandlerFunc) {
	if len(_config.AdminTokens) == 0 {
		next(c)
		return
	} else {
		key := c.RequestHeader("Authorization")
		if len(key) == 0 {
			c.Status(401)
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
			c.Status(401)
		}
	}
}
