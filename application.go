package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
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

type applocationLog struct {
	Version      string `json:"version"`
	Host         string `json:"host"`
	Level        int    `json:"level"`
	ShortMessage string `json:"short_message"`
	FullMessage  string `json:"full_message"`
	Timestamp    int64  `json:"timestamp"`
	RequestID    string `json:"_request_id"`
	Facility     string `json:"_facility"`
}

func writeApplicationLog(connectionString string) {
	url, err := url.Parse(connectionString)
	panicIf(err)
	var conn net.Conn
	if strings.EqualFold(url.Scheme, "tcp") {
		conn, err = net.Dial("tcp", url.Host)
		if err != nil {
			_logger.errorf("application log connection was failed %v", err)
		}
	} else {
		conn, err = net.Dial("udp", url.Host)
		if err != nil {
			_logger.errorf("application log connection was failed %v", err)
		}
	}

	// check connection status every 5 seconds
	var emptyByteArray []byte
	go func() {
		for {
			if conn != nil {
				_, err = conn.Write(emptyByteArray)
				if err != nil {
					conn = nil
				}
			} else {
				// TODO: tcp is hard-code, we need to remove that
				newConn, err := net.Dial("tcp", url.Host)
				if err == nil {
					conn = newConn
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	var empty byte
	for {
		select {
		case logElement := <-_errorLogsChan:
			go func(log applocationLog) {
				if conn != nil {
					payload, _ := json.Marshal(log)
					payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
					conn.Write(payload)
				}
			}(logElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
