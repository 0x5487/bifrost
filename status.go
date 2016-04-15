package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/jasonsoft/napnap"
)

type responseWriterWithLength struct {
	http.ResponseWriter
	length int
}

func (w *responseWriterWithLength) Write(b []byte) (n int, err error) {
	n, err = w.ResponseWriter.Write(b)
	w.length += n
	return
}

func (w *responseWriterWithLength) Length() int {
	return w.length
}

type status struct {
	sync.Mutex
	RequestCount int64     `json:"request_count"`
	NetworkIn    int64     `json:"network_in"`
	NetworkOut   int64     `json:"network_out"`
	StartAt      time.Time `json:"start_at"`
}

func newStatusMiddleware() *status {
	now := time.Now().UTC()
	return &status{
		StartAt: now,
	}
}

func (s *status) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	s.Lock()
	s.RequestCount++
	if c.Request.ContentLength > 0 {
		s.NetworkIn += c.Request.ContentLength
	}
	s.Unlock()

	lengthWriter := &responseWriterWithLength{c.Writer, 0}
	c.Writer = lengthWriter
	next(c)

	s.Lock()
	s.NetworkOut += int64(lengthWriter.length)
	s.Unlock()
}
