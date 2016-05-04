package main

import "github.com/jasonsoft/napnap"

type customErrorsMiddleware struct {
}

func newCustomErrorsMiddleware() *customErrorsMiddleware {
	return &customErrorsMiddleware{}
}

func (cem *customErrorsMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	next(c)
	code, _ := c.Get("status_code")
	if code != nil {
		statusCode := code.(int)
		if statusCode == 500 {
			appError := AppError{
				ErrorCode: "UNKNOWN_ERROR",
				Message:   "An unknown error has occurred.",
			}
			c.JSON(500, appError)
		}
	}

}
