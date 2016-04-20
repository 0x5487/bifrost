package main

import (
	"bytes"
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
	}
	return p
}

func (p *Proxy) Invoke(c *napnap.Context, next napnap.HandlerFunc) {
	_logger.debug("begin proxy")
	var api *Api
	requestHost := c.Request.URL.Host
	requestPath := c.Request.URL.Path
	consumer := c.Get("consumer").(Consumer)

	// find api entry which match the request.
	for _, apiEntry := range _config.Apis {
		// ensure request host is match
		if apiEntry.RequestHost != "*" {
			if requestHost != apiEntry.RequestHost {
				continue
			}
		}
		// ensure request path is match
		if strings.HasPrefix(requestPath, apiEntry.RequestPath) == false {
			continue
		}
		// ensure the consumer has access permission
		if apiEntry.isAllow(consumer) == false {
			if consumer.isAuthenticated() {
				c.Status(403)
				return
			}
			c.Status(401)
			return
		}
		api = &apiEntry
		break
	}

	// none of api enties are match
	if api == nil {
		next(c) // go to notFound middleware
		return
	}

	_logger.debugf("api host: %s", api.RequestHost)
	_logger.debugf("api path: %s", api.RequestPath)

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

	_logger.debugf("URL: %s", url)

	method := c.Request.Method
	body, _ := ioutil.ReadAll(c.Request.Body)

	outReq, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}

	// copy the request header
	p.copyHeader(outReq.Header, c.Request.Header)
	for _, h := range p.hopHeaders {
		outReq.Header.Del(h)
	}

	// forward reuqest ip
	if _config.ForwardRequestIP {
		clientIP := c.RemoteIpAddress()
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := outReq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outReq.Header.Set("X-Forwarded-For", clientIP)
	}

	// forward reuqest id
	if _config.ForwardRequestID {
		requestID := c.Get("request_id").(string)
		outReq.Header.Set("X-Request-Id", requestID)
	}

	// forward consumer information
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
		if len(consumer.CustomFields) > 0 {
			for key, field := range consumer.CustomFields {
				if len(key) > 0 && len(field) > 0 {
					outReq.Header.Set("X-Consumer-"+key, field)
				}
			}
		}
	}

	// send to target
	resp, err := p.client.Do(outReq)
	if err != nil {
		// upsteam server is down
		if strings.Contains(err.Error(), "No connection could be made") {
			c.Writer.WriteHeader(504)
			return
		}
		panic(err)
	}
	defer respClose(resp.Body)

	// copy the response header
	p.copyHeader(c.Writer.Header(), resp.Header)
	for _, h := range p.hopHeaders {
		c.Writer.Header().Del(h)
	}

	// write body
	body, _ = ioutil.ReadAll(resp.Body)
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