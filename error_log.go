package main

import (
	"fmt"
	"net"
	"net/http/httputil"
	"time"

	"github.com/jasonsoft/napnap"
)

type errorLog struct {
	host         string
	domain       string
	requestID    string
	shortMessage string
	fullMessage  string
	clientIP     string
}

type errorLogMiddleware struct {
	enableErrorLog bool
}

func newErrorLogMiddleware(enableErrorLog bool) *errorLogMiddleware {
	return &errorLogMiddleware{
		enableErrorLog: enableErrorLog,
	}
}

func (m *errorLogMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	defer func() {
		if r := recover(); r != nil {
			appError, ok := r.(AppError)
			if ok {
				c.Set("error", appError.Message)
				if appError.ErrorCode == "NOT_FOUND" {
					c.JSON(404, appError)
					return
				}
				c.JSON(400, appError)
				return
			}

			// unknown error
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("unknow error: %v", err)
			}
			_logger.debugf("unknown error: %v", err)
			c.Set("error", err.Error())
			c.JSON(500, err)

			// write error log
			if m.enableErrorLog {
				clientIP := getClientIP(c.RemoteIPAddress())
				requestID := c.MustGet("request-id").(string)
				requestDump, err := httputil.DumpRequest(c.Request, true)
				shortMsg := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
				fullMessage := fmt.Sprintf("error message: %s \n request info: %s \n ", err.Error(), string(requestDump))
				errorLog := errorLog{
					host:         _app.Hostname,
					domain:       c.Request.Host,
					requestID:    requestID,
					shortMessage: shortMsg,
					fullMessage:  fullMessage,
					clientIP:     clientIP,
				}

				select {
				case _errorLogsChan <- errorLog:
				default:
					_logger.debug("error queue was full")
				}
			}
		}
	}()
	next(c)
}

func writeErrorLog(connectionString string) {
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
		case logElement := <-_errorLogsChan:
			go func(log errorLog) {
				str := fmt.Sprintf(`{
				"host": "%s",
				"short_message": "%s",
				"full_message": "%s",
				"_request_id": "%s",
				"_domain": "%s",
				"_client_ip": "%s"
			}`, log.host, log.shortMessage, log.fullMessage, log.requestID, log.domain, log.clientIP)
				payload := []byte(str)
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				conn.Write(payload)
			}(logElement)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}
