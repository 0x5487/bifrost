package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/jasonsoft/napnap"
)

type Proxy struct {
	client     *http.Client
	hopHeaders []string
}

func NewProxy() *Proxy {
	p := &Proxy{}

	p.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
		},
		Timeout: time.Duration(5) * time.Second,
	}

	// Hop-by-hop headers. These are removed when sent to the backend.
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
	p.hopHeaders = []string{
		"Connection",
		"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",      // canonicalized version of "TE"
		"Trailer", // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
		"Transfer-Encoding",
		"Upgrade",

		// custom
		"Content-Length",
	}
	return p
}

func (p *Proxy) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	var api *Api
	requestHost := c.Request.URL.Host
	requestPath := c.Request.URL.Path

	// find api entry which match the request.  If no api entry is match, please ignore it.
	for _, apiEntry := range _config.Apis {
		// ensure request host is match
		if apiEntry.RequestHost != "*" {
			if requestHost != apiEntry.RequestHost {
				continue
			}
		}

		// ensure request path is match
		if strings.HasPrefix(requestPath, apiEntry.RequestPath) {
			api = &apiEntry
			break
		}
	}

	// none of api enties are match
	if api == nil {
		return
	}

	println("api host:", api.RequestHost)
	println("api path:", api.RequestPath)

	// exchange url
	var url string
	if api.StripRequestPath {
		newPath := strings.TrimPrefix(requestPath, api.RequestPath)
		url = api.TargetURL + newPath
	} else {
		url = api.TargetURL + requestPath
	}

	rawQuery := c.Request.URL.RawQuery
	if len(rawQuery) > 0 {
		url += "?" + rawQuery
	}

	println("URL:>", url)

	method := c.Request.Method
	outReq, err := http.NewRequest(method, url, c.Request.Body)

	// copy the request header
	p.copyHeader(outReq.Header, c.Request.Header)
	for _, h := range p.hopHeaders {
		outReq.Header.Del(h)
	}

	// send to target
	resp, err := p.client.Do(outReq)
	if err != nil {
		panic(err)
	}

	defer func() {
		// Drain and close the body to let the Transport reuse the connection
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	// copy the response header
	p.copyHeader(c.Writer.Header(), resp.Header)
	for _, h := range p.hopHeaders {
		c.Writer.Header().Del(h)
	}

	// forward reuqest ip
	if _config.Global.ForwardRequestIP {
		ip := c.RemoteIpAddress()
		c.Writer.Header().Set("X-Forwarded-For", ip)
	}

	// write body
	body, _ := ioutil.ReadAll(resp.Body)
	c.Writer.Write(body)
}

// CopyHeaders copies http headers from source to destination, it
// does not overide, but adds multiple headers
func (p *Proxy) copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
