package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/jasonsoft/napnap"
)

type accessLog struct {
	Host          string `json:"host"`
	ShortMessage  string `json:"short_message"`
	FullMessage   string `json:"full_message"`
	RequestID     string `json:"_request_id"`
	Domain        string `json:"_domain"`
	Status        int    `json:"_status"`
	ContentLength int    `json:"_content_length"`
	ClientIP      string `json:"_client_ip"`
	Duration      string `json:"_duration"`
	UserAgent     string `json:"_userAgent"`
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
	userAgnet := c.RequestHeader("User-Agent")
	accessLog := accessLog{
		Host:          _app.hostname,
		ShortMessage:  shortMsg,
		RequestID:     requestID,
		Domain:        c.Request.Host,
		Status:        c.Writer.Status(),
		ContentLength: c.Writer.ContentLength(),
		ClientIP:      clientIP,
		UserAgent:     userAgnet,
		Duration:      duration.String(),
	}

	if !(c.Writer.Status() >= 200 && c.Writer.Status() < 400) {
		requestDump, _ := httputil.DumpRequest(c.Request, true)
		respMsg, _ := c.Get("error")
		if respMsg != nil {
			respMessage := respMsg.(string)
			fullMessage := fmt.Sprintf("Upsteam response: %s \n\nRequest info: %s \n ", respMessage, string(requestDump))
			accessLog.FullMessage = fullMessage
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

	/*
		g := newGelf(gelfConfig{
			ConnectionString: connectionString,
		})
	*/
	var empty byte
	for {
		select {
		case logElement := <-_accessLogsChan:
			go func(log accessLog) {
				payload, _ := json.Marshal(log)
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				//g.log(payload)
				_logger.debugf("payload size: %v", len(payload))
				conn.Write(payload)
			}(logElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
