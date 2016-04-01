package main

import "net/http"

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var _hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

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
