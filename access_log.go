package main

import (
	"fmt"
	"net/http/httputil"
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
	accessLog := newGelfMessage(_app.hostname, _app.name, "access", 6)
	accessLog.CustomFields["request_id"] = c.MustGet("request-id").(string)
	accessLog.ShortMessage = fmt.Sprintf("%s %s [%d] %dms", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
	accessLog.CustomFields["request_host"] = c.Request.Host
	accessLog.CustomFields["path"] = c.Request.URL.Path
	accessLog.CustomFields["status"] = c.Writer.Status()
	accessLog.CustomFields["content_length"] = c.Writer.ContentLength()
	accessLog.CustomFields["client_ip"] = getClientIP(c.RemoteIPAddress())
	accessLog.CustomFields["user_agent"] = c.RequestHeader("User-Agent")
	accessLog.CustomFields["duration"] = duration

	cs, exist := c.Get("consumer")
	if exist {
		if consumer, ok := cs.(Consumer); ok && len(consumer.ID) > 0 {
			accessLog.CustomFields["consumer_id"] = consumer.ID
		}
	}

	if !(c.Writer.Status() >= 200 && c.Writer.Status() < 400) {
		requestDump, _ := httputil.DumpRequest(c.Request, true)
		respMsg, _ := c.Get("error")
		if respMsg != nil {
			respMessage := respMsg.(string)
			accessLog.FullMessage = fmt.Sprintf("Upsteam response: %s \n\nRequest info: %s \n ", respMessage, string(requestDump))
		}
	}

	select {
	case _messageChan <- accessLog:
	default:
		_logger.debug("message queue was full")
	}
}

func listQueueCount() {
	for {
		_logger.debug(fmt.Sprintf("count: %d", len(_messageChan)))
		time.Sleep(1 * time.Second)
	}
}
