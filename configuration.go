package main

import "errors"

var ErrDataAddr = errors.New("config: data address can't be empty")

type Header struct {
	AddHeader string
}

type TokenSetting struct {
	Timeout           int64 `yaml:"timeout"`
	VerifyIP          bool  `yaml:"verify_ip"`
	SlidingExpiration bool  `yaml:"sliding_expiration"`
}

type DataSetting struct {
	Type             string `yaml:"type"`
	ConnectionString string `yaml:"connection_string"`
	Address          string `yaml:"address"`
	Password         string `yaml:"password"`
	DB               string `yaml:"db"`
}

type Logs struct {
	ErrorLog string
}

type Configuration struct {
	Debug bool `yaml:"debug"`
	Logs  struct {
		Target struct {
			Name             string `yaml:"name"`
			Type             string `yaml:"type"`
			ConnectionString string `yaml:"connection_string"`
		} `yaml:"target"`
		AccessLog      bool `yaml:"access_log"`
		ApplicationLog bool `yaml:"application_log"`
	}
	CustomErrors     bool     `yaml:"custom_errors"`
	Binds            []string `yaml:"binds"`
	AdminTokens      []string `yaml:"admin_tokens"`
	ForwardRequestIP bool     `yaml:"forward_request_ip"`
	ForwardRequestID bool     `yaml:"forward_request_id"`
	Data             DataSetting
	Cors             struct {
		Enable bool `yaml:"enable"`
	}
	Gzip struct {
		Enable bool `yaml:"enable"`
	}
	Token TokenSetting
	TLS   struct {
		Enable               bool     `yaml:"enable"`
		Addr                 string   `yaml:"addr"`
		ApplyCertDomainNames []string `yaml:"apply_cert_domain_names"`
	}
}

func newConfiguration() Configuration {
	return Configuration{
		Binds: []string{":8080"},
		Data: DataSetting{
			Type: "memory",
		},
		Token: TokenSetting{
			Timeout: 1200, // 20 mins
		},
	}
}

func (c *Configuration) isValid() error {
	if c.Data.Type == "redis" {
		if len(c.Data.Address) == 0 {
			return ErrDataAddr
		}
	}
	return nil
}
