package main

import "net/http"

type Api struct {
	Name             string `json: name`
	RequestHost      string `json: rquest_host`
	RequestPath      string `json: request_path`
	StripRequestPath bool   `json: strip_request_path`
	TargetUrl        string `json: target_url`
}

// CopyHeaders copies http headers from source to destination, it
// does not overide, but adds multiple headers
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
