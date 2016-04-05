package main

type (
	Api struct {
		Name             string `yaml:"name"`
		RequestHost      string `yaml:"request_host"`
		RequestPath      string `yaml:"request_path"`
		StripRequestPath bool   `yaml:"strip_request_path"`
		TargetURL        string `yaml:"target_url"`
		Whitelist        string `yaml:"whitelist"`
		Blacklist        string `yaml:"blacklist"`
	}

	Configuration struct {
		Global struct {
			Cors struct {
				Enable         bool     `yaml:"enable"`
				AllowedOrigins []string `yaml:"allowed_origins"`
			}
			Gzip struct {
				Enable bool `yaml:"enable"`
			}
			ForwardRequestIP bool `yaml:"forward_request_ip"`
			Port             int  `yaml:"port"`
			Debug            bool `yaml:"debug"`
		}
		Apis []Api
	}
)
