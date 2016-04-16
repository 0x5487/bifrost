package main

import "strings"

type Policy struct {
	Allow string `yaml:"allow"`
	Deny  string `yaml:"deny"`
}

func (p Policy) isAllowPolicy() bool {
	if len(p.Allow) > 0 {
		return true
	}
	return false
}

func (p Policy) isMatch(kind string, consumer Consumer) bool {
	var rule string
	if kind == "deny" {
		rule = strings.ToLower(p.Deny)
	}

	if kind == "allow" {
		rule = strings.ToLower(p.Allow)
	}

	if rule == "all" {
		return true
	}
	terms := strings.Split(rule, ":")
	if terms[0] == "g" {
		for _, group := range consumer.Groups {
			if group == terms[1] {
				return true
			}
		}
	}
	return false
}

type Header struct {
	AddHeader string
}
type Api struct {
	Name             string   `yaml:"name"`
	RequestHost      string   `yaml:"request_host"`
	RequestPath      string   `yaml:"request_path"`
	StripRequestPath bool     `yaml:"strip_request_path"`
	TargetURL        string   `yaml:"target_url"`
	Policies         []Policy `yaml:"policies"`
}

func (a Api) isAllow(consumer Consumer) bool {
	for _, policy := range a.Policies {
		if policy.isAllowPolicy() == false {
			if policy.isMatch("deny", consumer) {
				return false
			}
		} else {
			if policy.isMatch("allow", consumer) {
				return true
			}
		}
	}
	// if there isn't any policies, return true
	return true
}

type Configuration struct {
	Debug            bool     `yaml:"debug"`
	Binds            []string `yaml:"binds"`
	AdminTokens      []string `yaml:"admin_tokens"`
	ForwardRequestIP bool     `yaml:"forward_request_ip"`
	ForwardRequestID bool     `yaml:"forward_requst_id"`
	Cors             struct {
		Enable         bool     `yaml:"enable"`
		AllowedOrigins []string `yaml:"allowed_origins"`
	}
	Gzip struct {
		Enable bool `yaml:"enable"`
	}
	Token TokenSetting
	Apis  []Api
}

func newConfiguration() Configuration {
	return Configuration{
		Binds: []string{":8080"},
		Token: TokenSetting{
			Timeout: 10,
		},
	}
}

func (c Configuration) isValid() error {
	// TODO: need to implement the features
	return nil
}

type TokenSetting struct {
	Timeout int
}
