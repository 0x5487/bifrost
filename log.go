package main

import (
	"log"

	"gopkg.in/mgo.v2"
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

// TODO: the following code need to be refactored
type loggerMongo struct {
}

func newloggerMongo() *loggerMongo {
	//create index
	return &loggerMongo{}
}

func (lm *loggerMongo) writeErrorLog(errorlog AppError) {
	session, err := mgo.Dial(_config.Logs.ErrorLog.ConnectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("error_log")

	err = c.Insert(errorlog)
	_logger.debug(err)
}
