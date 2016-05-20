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
	Version       string  `json:"version"`
	Host          string  `json:"host"`
	ShortMessage  string  `json:"short_message"`
	FullMessage   string  `json:"full_message"`
	Timestamp     float64 `json:"timestamp"`
	RequestID     string  `json:"_request_id"`
	Origin        string  `json:"_origin"`
	Path          string  `json:"_path"`
	Status        int     `json:"_status"`
	ContentLength int     `json:"_content_length"`
	ClientIP      string  `json:"_client_ip"`
	ConsumerID    string  `json:"_consumer_id"`
	Duration      int64   `json:"_duration"`
	UserAgent     string  `json:"_userAgent"`
}

func newAccessLog() *accessLog {
	return &accessLog{
		Version: "1.1",
	}
}

type accessLogMiddleware struct {
}

func newAccessLogMiddleware() *accessLogMiddleware {
	return &accessLogMiddleware{}
}

func (am *accessLogMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	startTime := time.Now()
	next(c)
	duration := int64(time.Since(startTime) / time.Millisecond)
	accessLog := accessLog{
		Version:       "1.1",
		Host:          _app.hostname,
		ShortMessage:  fmt.Sprintf("%s %s [%d] %dms", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration),
		Timestamp:     float64(time.Now().UnixNano()) / float64(time.Second),
		RequestID:     c.MustGet("request-id").(string),
		Origin:        c.RequestHeader("Origin"),
		Path:          c.Request.URL.Path,
		Status:        c.Writer.Status(),
		ContentLength: c.Writer.ContentLength(),
		ClientIP:      getClientIP(c.RemoteIPAddress()),
		UserAgent:     c.RequestHeader("User-Agent"),
		Duration:      duration,
	}

	cs, exist := c.Get("consumer")
	if exist {
		if consumer, ok := cs.(Consumer); ok && len(consumer.ID) > 0 {
			accessLog.ConsumerID = consumer.ID
		}
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
		if err != nil {
			_logger.errorf("access log connection was failed %v", err)
		}
	} else {
		conn, err = net.Dial("udp", url.Host)
		if err != nil {
			_logger.errorf("access log connection was failed %v", err)
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
				if conn != nil {
					payload, _ := json.Marshal(log)
					payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
					//g.log(payload)
					_logger.debugf("payload size: %v", len(payload))
					conn.Write(payload)
				}
			}(logElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
