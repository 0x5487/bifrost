package main

import (
	"sync"
	"time"

	"github.com/jasonsoft/napnap"
)

type status struct {
	sync.Mutex
	RequestCount   int64     `json:"request_count"`
	NetworkIn      int64     `json:"network_in"`
	NetworkOut     int64     `json:"network_out"`
	MemoryAcquired uint64    `json:"memory_acquired"`
	MemoryUsed     uint64    `json:"memory_used"`
	StartAt        time.Time `json:"start_at"`
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

	next(c)

	s.Lock()
	s.NetworkOut += int64(c.Writer.ContentLength())
	s.Unlock()
}
