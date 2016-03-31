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

		// forward request
		url := api.TargetUrl + requestPath

		if len(rawQuery) > 0 {
			url += "?" + rawQuery
		}

		fmt.Println("URL:>", url)
		req, err := http.NewRequest(method, url, c.Request.Body)

		// copy the request header
		//copyHeader(req.Header, c.Request.Header)

		// send to target
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// copy the response header
		c.Writer.WriteHeader(resp.StatusCode)
		//fmt.Println("response Headers:", resp.Header)
		//copyHeader(c.Writer.Header(), resp.Header)
		c.Writer.Header().Set("X-Forwarded-For", "127.0.2.31")

		//fmt.Println("response Body:", string(body))
		body, _ := ioutil.ReadAll(resp.Body)
		c.Writer.Write(body)
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})

	nap.Run(":8080")
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
