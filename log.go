package main

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"
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
		log.Println("[Debug] ", v)
	}
}

func (l *logger) debugf(format string, v ...interface{}) {
	if l.mode <= debugLevel {
		log.Printf("[Debug] "+format, v)
	}
}

func (l *logger) info(v ...interface{}) {
	if l.mode <= infoLevel {
		log.Println("[Info] ", v)
	}
}

func (l *logger) infof(format string, v ...interface{}) {
	if l.mode <= infoLevel {
		log.Printf("[Info] "+format, v)
	}
}

func (l *logger) error(v ...interface{}) {
	if l.mode <= errorLevel {
		log.Println("[Debug] ", v)
	}
}

func (l *logger) errorf(format string, v ...interface{}) {
	if l.mode <= errorLevel {
		log.Printf("[Error] "+format, v)
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

func writeAccessLog(connectionString string) {
	url, err := url.Parse(connectionString)
	panicIf(err)
	var conn net.Conn
	if strings.EqualFold(url.Scheme, "tcp") {
		conn, err = net.Dial("tcp", url.Host)
		if err != nil {
			_logger.errorf("access log connection was failed %v", err)
		}
	} else {
		conn, err = net.Dial("udp", url.Host)
		if err != nil {
			_logger.errorf("access log connection was failed %v", err)
		}
	}

	// check connection status every 1 second
	go func() {
		for {
			if conn == nil {
				// TODO: tcp is hard-code at the point, we need to remove that later
				newConn, err := net.Dial("tcp", url.Host)
				if err == nil {
					conn = newConn
					_logger.debug("created new connection")
				}
			} else {
				// check connection is alive
				one := []byte{}
				if _, err := conn.Read(one); err != nil {
					_logger.debugf("connection was closed: %v", err)
					conn.Close()
					conn = nil
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	var empty byte
	for {
		select {
		case message := <-_messageChan:
			if conn != nil {
				payload := message.toByte()
				payload = append(payload, empty) // when we use tcp, we need to add null byte in the end.
				wsize, err := conn.Write(payload)
				if err != nil {
					_logger.debugf("failed to write: %v", err)
					conn.Close()
					conn = nil
				} else {
					msg := fmt.Sprintf("[%s]payload size: %d", message.LoggerName, wsize)
					_logger.debug(msg)
				}
			}
		default:
			time.Sleep(3 * time.Second)
		}
	}
}
