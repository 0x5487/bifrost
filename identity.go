package main

import "github.com/jasonsoft/napnap"

func identity(c *napnap.Context, next napnap.HandlerFunc) {
	key := c.Request.Header.Get("Authorization")
	if len(key) == 0 {
		next(c)
	}
}
