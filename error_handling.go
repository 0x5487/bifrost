package main

import (
	"fmt"
	"net/http/httputil"

	"github.com/jasonsoft/napnap"
)

type applicationLogMiddleware struct {
	writeLog bool
}

func newApplicationLogMiddleware(writeLog bool) *applicationLogMiddleware {
	return &applicationLogMiddleware{
		writeLog: writeLog,
	}
}

func (m *applicationLogMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
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
			if m.writeLog {
				requestDump, _ := httputil.DumpRequest(c.Request, true)
				appLog := newGelfMessage(_app.hostname, _app.name, "applications", 3)
				appLog.CustomFields["request_id"] = c.MustGet("request-id").(string)
				appLog.ShortMessage = err.Error()
				appLog.FullMessage = fmt.Sprintf("request info: %s", string(requestDump))

				select {
				case _messageChan <- appLog:
				default:
					_logger.debug("message queue was full")
				}
			}
		}
	}()
	next(c)
}
