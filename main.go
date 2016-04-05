package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/jasonsoft/napnap"
	"gopkg.in/yaml.v2"
)

var (
	_config Configuration
	_logger *logger
)

func init() {
	flag.Parse()

	//read and parse config file
	var err error
	rootDirPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("file error: %v", err)
	}

	configPath := filepath.Join(rootDirPath, "config.yml")
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("file error: %v", err)
	}

	err = yaml.Unmarshal(file, &_config)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	apis := _config.Apis
	if len(apis) == 0 {
		log.Fatalf("config error: no api entries were found.")
	}

	// setup logger
	_logger = newLog()
	if _config.Global.Debug {
		_logger.mode = Debug
	}
}

func main() {

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

	// admin api
	adminNap := napnap.New()
	adminNap.Use(napnap.NewHealth())
	adminRouter := napnap.NewRouter()
	// verify all request which send to admin api and ensure the caller has valid admin token.
	adminRouter.All("/v1/admin", authAdminEndpoint)
	adminNap.Use(adminRouter)

	consumerRouter := napnap.NewRouter()
	consumerRouter.Post("/v1/admin/consumers", createConsumerEndpoint)
	consumerRouter.Get("/v1/admin/consumers", getConsumerEndpoint)
	consumerRouter.Delete("/v1/admin/consumers", deletedConsumerEndpoint)
	adminNap.Use(consumerRouter)

	// run two http servers on different ports
	// one is for bifrost service and another is for admin api
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// http server for admin api
		err := adminNap.Run(":8001")
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// http server for bifrost service
		portValue := fmt.Sprintf(":%d", port)
		err := nap.Run(portValue)
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()
	wg.Wait()
}
