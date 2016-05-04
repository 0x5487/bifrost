package main

import (
	"fmt"
	"net"
	"net/http/httputil"
	"time"

	"github.com/jasonsoft/napnap"
)

type accessLog struct {
	host          string
	shortMessage  string
	fullMessage   string
	requestID     string
	domain        string
	status        int
	contentLength int
	clientIP      string
	duration      string
	userAgent     string
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

	clientIP := getClientIP(c.RemoteIPAddress())
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
		duration:      duration.String(),
	}

	if !(c.Writer.Status() >= 200 && c.Writer.Status() < 400) {
		requestDump, _ := httputil.DumpRequest(c.Request, true)
		respMsg, _ := c.Get("error")
		if respMsg != nil {
			respMessage := respMsg.(string)
			fullMessage := fmt.Sprintf("Upsteam response: %s \n\nRequest info: %s \n ", respMessage, string(requestDump))
			accessLog.fullMessage = fullMessage
		}
	}

	select {
	case _accessLogsChan <- accessLog:
	default:
		_logger.debug("access log queue was full")
	}
}

func listQueueCount() {
	for {
		_logger.debug(fmt.Sprintf("count: %d", len(_accessLogsChan)))
		time.Sleep(1 * time.Second)
	}
}

func writeAccessLog(connectionString string) {
	conn, err := net.Dial("tcp", connectionString)
	if err != nil {
		panic(err)
	}

	// check connection status every 5 seconds
	hi := []byte("hi")
	go func() {
		for {
			_, err = conn.Write(hi)
			if err != nil {
				newConn, err := net.Dial("tcp", connectionString)
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
		case accesslogElement := <-_accessLogsChan:
			go func(log accessLog) {
				str := fmt.Sprintf(`{
				"host": "%s",
				"short_message": "%s",
				"full_message": "%s",
				"_request_id": "%s",
				"_domain": "%s",
				"_status": %d,
				"_content_length" : %d,
				"_client_ip": "%s",
				"_duration": "%s"
			}`, log.host, log.shortMessage, log.fullMessage, log.requestID, log.domain, log.status, log.contentLength, log.clientIP, log.duration)
				payload := []byte(str)
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				conn.Write(payload)
			}(accesslogElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
