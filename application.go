package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jasonsoft/napnap"
)

type status struct {
	Hostname       string    `json:"hostname"`
	NumCPU         int       `json:"cpu_core"`
	TotalRequests  uint64    `json:"total_requests"`
	NetworkIn      int64     `json:"network_in"`
	NetworkOut     int64     `json:"network_out"`
	MemoryAcquired uint64    `json:"memory_acquired"`
	MemoryUsed     uint64    `json:"memory_used"`
	StartAt        time.Time `json:"start_at"`
	Uptime         string    `json:"uptime"`
}

type application struct {
	sync.Mutex
	name          string
	hostname      string
	totalRequests uint64
	networkIn     int64
	networkOut    int64
	startAt       time.Time
}

func newApplication() *application {
	name, err := os.Hostname()
	panicIf(err)

	return &application{
		name:     "bifrost",
		hostname: name,
		startAt:  time.Now().UTC(),
	}
}

func (a *application) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	a.Lock()
	a.totalRequests++
	if c.Request.ContentLength > 0 {
		a.networkIn += c.Request.ContentLength
	}
	a.Unlock()

	next(c)

	a.Lock()
	a.networkOut += int64(c.Writer.ContentLength())
	a.Unlock()
}

func notFound(c *napnap.Context, next napnap.HandlerFunc) {
	_logger.debug("not found")
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
	ErrorCode string `json:"error_code" bson:"-"`
	Message   string `json:"message" bson:"message"`
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s - %s", e.ErrorCode, e.Message)
}

type ApiCount struct {
	Count int `json:"count"`
}
