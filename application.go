package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jasonsoft/napnap"
)

type application struct {
	name     string
	hostname string
}

func newApplication() *application {
	name, err := os.Hostname()
	panicIf(err)

	return &application{
		name:     "bifrost",
		hostname: name,
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

func reload() []*service {
	services, err := _serviceRepo.GetAll()
	panicIf(err)
	// stop all health checking
	for _, svc := range _services {
		for _, upstream := range svc.Upstreams {
			upstream.stopChecking()
		}
	}
	for _, svc := range services {
		for _, upstream := range svc.Upstreams {
			upstream.startChecking()
		}
	}
	return services
}

func reloadUpstreams() {
	for _, svc := range _services {
		newSvc, err := _serviceRepo.Get(svc.ID)
		panicIf(err)
		for _, upstream := range svc.Upstreams {
			upstream.stopChecking()
		}
		svc.Upstreams = newSvc.Upstreams
		for _, upstream := range svc.Upstreams {
			upstream.startChecking()
		}
	}
}

type applocationLog struct {
	Host         string `json:"host"`
	Level        int    `json:"level"`
	ShortMessage string `json:"short_message"`
	FullMessage  string `json:"full_message"`
	RequestID    string `json:"_request_id"`
	App          string `json:"_app"`
	Domain       string `json:"_domain"`
	ClientIP     string `json:"_client_ip"`
}

func writeApplicationLog(connectionString string) {
	url, err := url.Parse(connectionString)
	panicIf(err)
	var conn net.Conn
	if strings.EqualFold(url.Scheme, "tcp") {
		conn, err = net.Dial("tcp", url.Host)
		panicIf(err)
	} else {
		conn, err = net.Dial("udp", url.Host)
		panicIf(err)
	}

	// check connection status every 5 seconds
	var emptyByteArray []byte
	go func() {
		for {
			_, err = conn.Write(emptyByteArray)
			if err != nil {
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
				payload, _ := json.Marshal(log)
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				conn.Write(payload)
			}(logElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
