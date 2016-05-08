package main

import (
	"fmt"
	"net/http/httputil"

	"github.com/jasonsoft/napnap"
)

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
		// we only handle error for bifrost application and don't handle can't error from upstream.
		if r := recover(); r != nil {
			// bad request.  http status code is 400 series.
			appError, ok := r.(AppError)
			if ok {
				c.Set("error", appError.Message)
				if appError.ErrorCode == "not_found" {
					c.JSON(404, appError)
					return
				}
				c.JSON(400, appError)
				return
			}

			// unknown error.  http status code is 500 series.
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
				errorLog := applocationLog{
					Host:         _app.hostname,
					App:          _app.name,
					Domain:       c.Request.Host,
					Level:        3,
					RequestID:    requestID,
					ShortMessage: shortMsg,
					FullMessage:  fullMessage,
					ClientIP:     clientIP,
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