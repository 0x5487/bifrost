package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/jasonsoft/napnap"
)

type proxy struct {
	client      *http.Client
	hopHeaders  []string
	corsHeaders []string
}

func newProxy() *proxy {
	p := &proxy{}

	p.client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
		},
		Timeout: time.Duration(30) * time.Second,
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
		"Cache-Control",
	}

	p.corsHeaders = []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods",
	}

	return p
}

func (p *proxy) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	_logger.debugf("request host: %v", c.Request.Host)
	_logger.debugf("request path: %v", c.Request.URL.Path)

	//requestHost := strings.ToLower(c.Request.Host)
	requestPath := strings.ToLower(c.Request.URL.Path)

	consumer := c.MustGet("consumer").(Consumer)

	// find api entry which match the request.
	var apiEntry *api
	for _, apiElement := range _apis {
		// ensure request host is match
		if apiElement.RequestHost != "*" && !strings.EqualFold(apiElement.RequestHost, c.Request.Host) {
			continue
		}
		// ensure request path is match
		if apiElement.RequestPath != "*" && strings.HasPrefix(requestPath, apiElement.RequestPath) == false {
			continue
		}
		// ensure the consumer has access permission
		if apiElement.isAllow(consumer) == false {
			if consumer.isAuthenticated() {
				c.SetStatus(403)
				return
			}
			c.SetStatus(401)
			return
		}
		apiEntry = apiElement
		break
	}

	// none of api enties are match
	if apiEntry == nil {
		next(c) // go to notFound middleware
		return
	}

	_logger.debugf("api host: %s", apiEntry.RequestHost)
	_logger.debugf("api path: %s", apiEntry.RequestPath)

	var targetURL string
	var svcEntry *service
	var upstreamEntry *upstream
	if len(apiEntry.Service) > 0 {
		for _, svcElement := range _services {
			if apiEntry.Service == svcElement.Name {
				svcEntry = svcElement
			}
		}
		if svcEntry != nil {
			// get upstream and exchange url
			upstreamEntry = svcEntry.askForUpstream()
			if upstreamEntry != nil {
				_logger.debugf("upstream: %v", upstreamEntry.Name)
				targetURL = upstreamEntry.TargetURL
			}

		}
	}

	if (svcEntry == nil || upstreamEntry == nil) && len(apiEntry.TargetURL) > 0 {
		_logger.debugf("api entry target url: %v", apiEntry.TargetURL)
		targetURL = apiEntry.TargetURL
	}

	if len(targetURL) == 0 {
		// no upstreams are available
		c.SetStatus(503)
		return
	}

	var url string
	if apiEntry.StripRequestPath {
		prefix := strings.ToLower(apiEntry.RequestPath)
		newPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, prefix) {
			newPath = c.Request.URL.Path[len(prefix):]
		}
		url = targetURL + newPath
	} else {
		url = targetURL + c.Request.URL.Path
	}

	rawQuery := c.Request.URL.RawQuery
	if len(rawQuery) > 0 {
		url += "?" + rawQuery
	}

	_logger.debugf("URL: %s", url)

	// redirect if needed
	if apiEntry.Redirect {
		_logger.debug("redirect to ", url)
		c.Redirect(301, url)
		return
	}

	method := c.Request.Method
	body, _ := ioutil.ReadAll(c.Request.Body)

	outReq, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}

	// copy the request header
	p.copyHeader(outReq.Header, c.Request.Header)
	p.removeHeader(outReq.Header)

	// forward reuqest ip
	if _config.ForwardRequestIP {
		clientIP := getClientIP(c.RemoteIPAddress())
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	// forward reuqest id
	if _config.ForwardRequestID {
		requestID := c.MustGet("request-id").(string)
		outReq.Header.Set("X-Request-Id", requestID)
	}

	// forward consumer information
	for k := range outReq.Header {
		// delete all header value start with X-Consumer
		if strings.HasPrefix(k, "X-Consumer") {
			outReq.Header.Del(k)
		}
	}

	if len(consumer.ID) > 0 {
		outReq.Header.Set("X-Consumer-Id", consumer.ID)
		if len(consumer.App) > 0 {
			outReq.Header.Set("X-Consumer-App", consumer.App)
		}
		if len(consumer.Username) > 0 {
			outReq.Header.Set("X-Consumer-Username", consumer.Username)
		}
		if len(consumer.CustomID) > 0 {
			outReq.Header.Set("X-Consumer-Custom-Id", consumer.CustomID)
		}
		if len(consumer.Roles) > 0 {
			roles := strings.Join(consumer.Roles, ",")
			outReq.Header.Set("X-Consumer-Roles", roles)
		}
		if len(consumer.CustomFields) > 0 {
			for key, field := range consumer.CustomFields {
				if len(key) > 0 && len(field) > 0 {
					outReq.Header.Set("X-Consumer-"+key, field)
				}
			}
		}
	}

	// forward token
	val, ok := c.Get("token")
	token, ook := val.(string)
	if ok && ook && len(token) > 0 {
		outReq.Header.Set("X-Token", token)
	}

	// send to target
	resp, err := p.client.Do(outReq)
	if err != nil {
		// upsteam server is down
		if strings.Contains(err.Error(), "No connection could be made") {
			if svcEntry != nil && upstreamEntry != nil {
				svcEntry.unregisterUpstream(upstreamEntry)
				p.Invoke(c, next) // resend
				return
			}
			c.SetStatus(504)
			return
		}
		// upstream server is timeout
		if strings.Contains(err.Error(), "request canceled") {
			_logger.debug("request canceled")
			c.SetStatus(504)
			return
		}
		panic(err)
	}
	defer respClose(resp.Body)

	body, _ = ioutil.ReadAll(resp.Body)

	// set error message
	if !(resp.StatusCode >= 200 && resp.StatusCode < 400) {
		c.Set("status_code", resp.StatusCode)
		err := string(body)
		c.Set("error", err)
		_logger.debugf("error: %v", err)
	}

	if _config.CustomErrors && resp.StatusCode == 500 {
		// don't write the message when custom error turns on
		return
	}

	// copy the response header
	p.removeHeader(resp.Header)
	p.copyHeader(c.Writer.Header(), resp.Header)

	// write body
	c.SetStatus(resp.StatusCode)
	c.Writer.Write(body)
}

// CopyHeaders copies http headers from source to destination, it
// does not overide, but adds multiple headers
func (p *proxy) copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (p *proxy) removeHeader(header http.Header) {
	for _, h := range p.hopHeaders {
		header.Del(h)
	}
	if _config.Cors.Enable {
		for _, corsHeader := range p.corsHeaders {
			header.Del(corsHeader)
		}
	}
}
