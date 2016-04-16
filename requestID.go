package main

import (
	"github.com/jasonsoft/napnap"
	"github.com/satori/go.uuid"
)

func requestIDMiddleware() napnap.MiddlewareFunc {
	return func(c *napnap.Context, next napnap.HandlerFunc) {
		requestID := uuid.NewV4().String()
		c.RespHeader("X-Request-Id", requestID)
		c.Set("request_id", requestID)
		next(c)
	}
}
