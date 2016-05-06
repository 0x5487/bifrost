package main

type Header struct {
	AddHeader string
}

type TokenSetting struct {
	Timeout           int  `yaml:"timeout"`
	VerifyIP          bool `yaml:"verify_ip"`
	SlidingExpiration bool `yaml:"sliding_expiration"`
}

type DataSetting struct {
	Type             string
	ConnectionString string `yaml:"connection_string"`
}

type Logs struct {
	ErrorLog string
}

type Configuration struct {
	Debug bool `yaml:"debug"`
	Logs  struct {
		AccessLog struct {
			Type             string `yaml:"type"`
			ConnectionString string `yaml:"connection_string"`
		} `yaml:"access_log"`
		ErrorLog struct {
			Type             string `yaml:"type"`
			ConnectionString string `yaml:"connection_string"`
		} `yaml:"error_log"`
	}
	CustomErrors     bool     `yaml:"custom_errors"`
	Binds            []string `yaml:"binds"`
	AdminTokens      []string `yaml:"admin_tokens"`
	ForwardRequestIP bool     `yaml:"forward_request_ip"`
	ForwardRequestID bool     `yaml:"forward_requst_id"`
	Data             DataSetting
	Cors             struct {
		Enable         bool     `yaml:"enable"`
		AllowedOrigins []string `yaml:"allowed_origins"`
	}
	Gzip struct {
		Enable bool `yaml:"enable"`
	}
	Token TokenSetting
}

func newConfiguration() Configuration {
	return Configuration{
		Binds: []string{":8080"},
		Data: DataSetting{
			Type: "memory",
		},
		Token: TokenSetting{
			Timeout: 10,
		},
	}
}

func (c Configuration) isValid() error {
	// TODO: need to implement the features
	return nil
}
