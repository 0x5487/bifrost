package main

import (
	"log"
)

const (
	Debug   = 0
	Info    = 1
	Warning = 2
	Error   = 3
	Fatal   = 4
)

type logger struct {
	mode int
}

func newLog() *logger {
	return &logger{
		mode: Info,
	}
}

func (l *logger) debug(v ...interface{}) {
	if l.mode <= Debug {
		log.Println(v)
	}
}

func (l *logger) debugf(format string, v ...interface{}) {
	if l.mode <= Debug {
		log.Printf(format, v)
	}
}

func (l *logger) fatal(v ...interface{}) {
	if l.mode <= Fatal {
		log.Fatal(v)
	}
}

func (l *logger) fatalf(format string, v ...interface{}) {
	if l.mode <= Fatal {
		log.Fatalf(format, v)
	}
}
