package main

import (
	"fmt"
	"net"
	"time"

	"github.com/jasonsoft/napnap"
)

type accessLog struct {
	host          string
	shortMessage  string
	requestID     string
	domain        string
	status        int
	contentLength int
	clientIP      string
	duration	string
}

type accessLogMiddleware struct {
}

func newAccessLogMiddleware() *accessLogMiddleware {
	return &accessLogMiddleware{}
}

func (am *accessLogMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	startTime := time.Now()
	next(c)
	duration := time.Since(startTime)

	clientIP := c.RemoteIPAddress()
	if len(clientIP) > 0 && clientIP == "::1" {
		clientIP = "127.0.0.1"
	}
	
	requestID := c.MustGet("request-id").(string)
	shortMsg := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
	accessLog := accessLog{
		host:          _app.Hostname,
		shortMessage:  shortMsg,
		requestID:     requestID,
		domain:        c.Request.Host,
		status:        c.Writer.Status(),
		contentLength: c.Writer.ContentLength(),
		clientIP:      clientIP,
		duration: duration.String(),
	}

	select {
	case _accessLogsChan <- accessLog:
	default:
		fmt.Println("queue full")
	}
}

func listQueueCount() {
	for {
		println(fmt.Sprintf("count: %d", len(_accessLogsChan)))
		time.Sleep(1 * time.Second)
	}
}

func writeAccessLog(connectionString string) {
	conn, err := net.Dial("tcp", connectionString)
	if err != nil {
		panic(err)
	}

	// check connection status
	hi := []byte("hi")
	go func() {
		for {
			_, err = conn.Write(hi)
			if err != nil {
				_logger.debug("conn was closed")
				newConn, err := net.Dial("tcp", connectionString)
				if err == nil {
					conn = newConn
				}
			} else {
				_logger.debug("conn is good")
			}
			time.Sleep(5 * time.Second)
		}
	}()

	var empty byte
	for {
		select {
		case accesslogElement := <-_accessLogsChan:
			go func(log accessLog) {
				str := fmt.Sprintf(`{
				"host": "%s",
				"short_message": "%s",
				"_request_id": "%s",
				"_domain": "%s",
				"_status": %d,
				"_content_length" : %d,
				"_client_ip": "%s",
				"_duration": "%s"
			}`, log.host, log.shortMessage, log.requestID, log.domain, log.status, log.contentLength, log.clientIP, log.duration)
				payload := []byte(str)
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				conn.Write(payload)
			}(accesslogElement)
		default:
			_logger.debug("write log is sleeping...")
			time.Sleep(5 * time.Second)
		}
	}
}
