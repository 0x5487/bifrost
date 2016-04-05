package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/jasonsoft/napnap"
	"gopkg.in/yaml.v2"
)

var (
	_config Configuration
)

func init() {
	flag.Parse()
	//read config file
	var err error
	rootDirPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		println(err)
		panic(err)
	}

	configPath := filepath.Join(rootDirPath, "config.yml")
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		fmt.Printf("file error: %v\n", err)
		os.Exit(1)
	}

	// yml
	err = yaml.Unmarshal(file, &_config)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	/*
		err = json.Unmarshal(file, &_config)
		if err != nil {
			fmt.Printf("config error: %v\n", err)
			os.Exit(1)
		}*/
}

func main() {

	apis := _config.Apis
	if len(apis) == 0 {
		panic("no api entries are found.")
	}

	/*
		api1 := Api{}
		api1.Name = "Test"
		api1.RequestHost = "*"
		_config.Apis = append(_config.Apis, api1)

		api2 := Api{}
		api2.Name = "Test"
		api2.RequestHost = "*"
		_config.Apis = append(_config.Apis, api2)

		d, err := yaml.Marshal(&_config)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		println(string(d))
	*/

	nap := napnap.New()

	// turn on gzip feature
	gzip := _config.Global.Gzip
	if gzip.Enable {
		nap.Use(napnap.NewGzip(napnap.DefaultCompression))
	}

	// turn on health check feature
	nap.Use(napnap.NewHealth())

	// turn on CORS feature
	cors := _config.Global.Cors
	if cors.Enable {
		options := napnap.Options{}
		options.AllowedOrigins = cors.AllowedOrigins
		nap.Use(napnap.NewCors(options))
	}

	nap.Use(NewProxy())

	// assign port number
	port := _config.Global.Port
	if port > 0 && port < 65535 {

	} else {
		port = 8080
	}

	portValue := fmt.Sprintf(":%d", port)
	err := nap.Run(portValue)
	if err != nil {
		println(err.Error())
	}

}
