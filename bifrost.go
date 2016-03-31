package main

type Api struct {
	Name             string `json: name`
	RequestHost      string `json: rquest_host`
	RequestPath      string `json: request_path`
	StripRequestPath bool `json: strip_request_path`
	TargetUrl        string `json: target_url`
}
