package main

import (
	"log"

	"gopkg.in/mgo.v2"
)

const (
	debugLevel   = 0
	infoLevel    = 1
	warningLevel = 2
	errorLevel   = 3
	fatalLevel   = 4
)

type logger struct {
	mode int
}

func newLog() *logger {
	return &logger{
		mode: infoLevel,
	}
}

func (l *logger) debug(v ...interface{}) {
	if l.mode <= debugLevel {
		log.Println(v)
	}
}

func (l *logger) debugf(format string, v ...interface{}) {
	if l.mode <= debugLevel {
		log.Printf(format, v)
	}
}

func (l *logger) info(v ...interface{}) {
	if l.mode <= infoLevel {
		log.Println(v)
	}
}

func (l *logger) infof(format string, v ...interface{}) {
	if l.mode <= infoLevel {
		log.Printf(format, v)
	}
}

func (l *logger) fatal(v ...interface{}) {
	if l.mode <= fatalLevel {
		log.Fatal(v)
	}
}

func (l *logger) fatalf(format string, v ...interface{}) {
	if l.mode <= fatalLevel {
		log.Fatalf(format, v)
	}
}

// TODO: the following code needs to be refactored
type loggerMongo struct {
}

func newloggerMongo() *loggerMongo {
	//create index
	return &loggerMongo{}
}

func (lm *loggerMongo) writeErrorLog(errorlog AppError) {
	session, err := mgo.Dial(_config.Logs.ApplicationLog.ConnectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("error_log")

	err = c.Insert(errorlog)
	if err != nil {
		_logger.debug(err)
	}
}
