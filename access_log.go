package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jasonsoft/napnap"
)

type accessLog struct {
	host          string
	shortMessage  string
	requestID     string
	domain        string
	status        int
	contentLength int
	clientIP      string
}

type accessLogMiddleware struct {
	gelf   *gelf
	conn   net.Conn
	client *http.Client
}

func newAccessLogMiddleware(connectionString string) *accessLogMiddleware {
	g := newGelf(gelfConfig{
		ConnectionString: connectionString,
		Connection:       "lan",
	})

	udpConn, err := net.Dial("tcp", connectionString)
	if err != nil {
		panic(err)
	}

	client1 := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
		},
		Timeout: time.Duration(30) * time.Second,
	}

	return &accessLogMiddleware{
		gelf:   g,
		conn:   udpConn,
		client: client1,
	}
}

func (am *accessLogMiddleware) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	next(c)

	clientIP := c.RemoteIPAddress()
	if len(clientIP) > 0 && clientIP == "::1" {
		clientIP = "127.0.0.1"
	}
	_logger.debug(clientIP)

	requestID := c.MustGet("request-id").(string)

	shortMsg := fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
	str := fmt.Sprintf(`{
				"host": "%s",
				"short_message": "%s",
				"_request_id": "%s",
				"_domain": "%s",
				"_status": %d,
				"_content_length" : %d,
				"_client_ip": "%s"
			}\00`, _app.Hostname, shortMsg, requestID, c.Request.Host, c.Writer.Status(), c.Writer.ContentLength(), clientIP)
	_logger.debugf("gelf message: %s", str)

	accessLog := accessLog{
		host:          _app.Hostname,
		shortMessage:  shortMsg,
		requestID:     requestID,
		domain:        c.Request.Host,
		status:        c.Writer.Status(),
		contentLength: c.Writer.ContentLength(),
		clientIP:      clientIP,
	}

	select {
	case _accessLogChan <- accessLog:
	default:
		fmt.Println("queue full")
	}

	/*
		go func(str string) {
			time.Sleep(5 * time.Second)

			// tcp or udp
			bb := []byte(str)
			var aa byte
			bb = append(bb, aa) // when we use tcp, we need to add null byte in the end.
			_, err := am.conn.Write(bb)
			if err != nil {
				println(err.Error())
			}

			//http
			/*
				outReq, err := http.NewRequest("POST", "http://192.168.1.2:12201/gelf", strings.NewReader(str))
				if err != nil {
					panic(err)
				}
				resp, err := am.client.Do(outReq)
				if err != nil {
					println(err.Error())
				}
				defer respClose(resp.Body)
	*/
	//}(str)
	//go am.gelf.log(str)

	/*
		go func(str string) {
			_logger.debugf("gelf message: %s", str)
			//am.gelf.log(str)
		}(str)
	*/
}

func listQueueCount() {
	for {
		println(fmt.Sprintf("count: %d", len(_accessLogChan)))
		time.Sleep(1 * time.Second)
	}
}

func writeAccessLog(connectionString string) {

	conn, err := net.Dial("tcp", connectionString)
	if err != nil {
		panic(err)
	}
	conn.Read()
	for {
		select {
		case accesslogElement := <-_accessLogChan:
			go func(log accessLog) {
				str := fmt.Sprintf(`{
				"host": "%s",
				"short_message": "%s",
				"_request_id": "%s",
				"_domain": "%s",
				"_status": %d,
				"_content_length" : %d,
				"_client_ip": "%s"
			}`, log.host, log.shortMessage, log.requestID, log.domain, log.status, log.contentLength, log.clientIP)
				bb := []byte(str)
				var aa byte
				bb = append(bb, aa) // when we use tcp, we need to add null byte in the end.
				_, err := conn.Write(bb)
				if err != nil {
					println(err.Error())
				}
			}(accesslogElement)
		default:
			_logger.debug("write log is sleeping...")
			time.Sleep(5 * time.Second)
		}
	}
}
