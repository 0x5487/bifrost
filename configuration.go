package main

type (
	Policy struct {
		Allow []string `yaml:"allow"`
		Deny  []string `yaml:"deny"`
	}
	Api struct {
		Name             string   `yaml:"name"`
		RequestHost      string   `yaml:"request_host"`
		RequestPath      string   `yaml:"request_path"`
		StripRequestPath bool     `yaml:"strip_request_path"`
		TargetURL        string   `yaml:"target_url"`
		Policies         []Policy `yaml:"policies"`
	}

	Configuration struct {
		AdminTokens      []string `yaml:"admin_tokens"`
		ForwardRequestIP bool     `yaml:"forward_request_ip"`
		Port             int      `yaml:"port"`
		Debug            bool     `yaml:"debug"`
		Cors             struct {
			Enable         bool     `yaml:"enable"`
			AllowedOrigins []string `yaml:"allowed_origins"`
		}
		Gzip struct {
			Enable bool `yaml:"enable"`
		}
		Apis []Api
	}
)
