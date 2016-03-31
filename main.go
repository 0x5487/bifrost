package main

import (
	"fmt"
	"io/ioutil"
	"napnap"
	"net"
	"net/http"
)

var (
	_apis []Api
)

func init() {
	api := Api{
		Name:             "Test",
		RequestHost:      "localhost",
		RequestPath:      "/",
		StripRequestPath: true,
	}

	_apis = append(_apis, api)
}

func main() {
	nap := napnap.New()

	nap.UseFunc(func(c *napnap.Context, next napnap.HandlerFunc) {
		api := Api{
			Name:             "Test",
			RequestHost:      "localhost",
			RequestPath:      "/",
			StripRequestPath: true,
			TargetUrl:        "https://tw.yahoo.com",
		}

		// get information
		requestPath := c.Request.URL.Path
		method := c.Request.Method
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		println("ip:" + ip)
		rawQuery := c.Request.URL.RawQuery

		// exchange url
		url := api.TargetUrl + requestPath

		if len(rawQuery) > 0 {
			url += "?" + rawQuery
		}

		fmt.Println("URL:>", url)
		req, err := http.NewRequest(method, url, c.Request.Body)

		// copy the request header
		copyHeader(req.Header, c.Request.Header)

		// send to target
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// copy the response header
		copyHeader(c.Writer.Header(), resp.Header)
		c.Writer.Header().Set("X-Forwarded-For", "127.0.2.32")

		// write body
		body, _ := ioutil.ReadAll(resp.Body)
		c.Writer.Write(body)
	})

	nap.Run(":8080")
}

