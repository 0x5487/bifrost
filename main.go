package main

import (
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var (
	_apis  []Api
	client *http.Client
)

const (
	MaxIdleConnections int = 8
	RequestTimeout     int = 5
)

func init() {
	api := Api{
		Name:             "Test",
		RequestHost:      "localhost",
		RequestPath:      "/",
		StripRequestPath: true,
	}

	_apis = append(_apis, api)
	client = createHTTPClient()
}

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: MaxIdleConnections,
		},
		Timeout: time.Duration(RequestTimeout) * time.Second,
	}

	return client
}

func proxy(w http.ResponseWriter, r *http.Request) {
	api := Api{
		Name:             "Test",
		RequestHost:      "localhost",
		RequestPath:      "/json",
		StripRequestPath: false,
		TargetUrl:        "http://localhost:8000",
	}

	// if the request url doesn't match, we will by pass it
	requestPath := r.URL.Path
	if !strings.HasPrefix(requestPath, api.RequestPath) {
		return
	}

	// get information
	//ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
	//println("ip:" + ip)

	// exchange url
	var url string
	if api.StripRequestPath {
		newPath := strings.TrimPrefix(requestPath, api.RequestPath)
		url = api.TargetUrl + newPath
	} else {
		url = api.TargetUrl + requestPath
	}

	rawQuery := r.URL.RawQuery
	if len(rawQuery) > 0 {
		url += "?" + rawQuery
	}

	//println("URL:>", url)

	method := r.Method
	req, err := http.NewRequest(method, url, r.Body)

	// copy the request header
	copyHeader(req.Header, r.Header)
	for _, h := range _hopHeaders {
		req.Header.Del(h)
	}

	// send to target
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// copy the response header
	copyHeader(w.Header(), resp.Header)
	//c.Writer.Header().Set("X-Forwarded-For", "127.0.2.32")
	for _, h := range _hopHeaders {
		w.Header().Del(h)
	}

	// write body
	body, _ := ioutil.ReadAll(resp.Body)
	w.Write(body)
}

func main() {
	http.HandleFunc("/", proxy)
	http.ListenAndServe(":8080", nil)
}
