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
	_config       Configuration
	_logger       *logger
	_consumerRepo ConsumerRepository
	_tokenRepo    TokenRepository
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
	if _config.Debug {
		_logger.mode = Debug
	}

	_consumerRepo = newConsumerMemStore()
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
	gzip := _config.Gzip
	if gzip.Enable {
		nap.Use(napnap.NewGzip(napnap.DefaultCompression))
	}

	// turn on health check feature
	nap.Use(napnap.NewHealth())

	// turn on CORS feature
	cors := _config.Cors
	if cors.Enable {
		options := napnap.Options{}
		options.AllowedOrigins = cors.AllowedOrigins
		nap.Use(napnap.NewCors(options))
	}

	nap.UseFunc(identity)
	nap.Use(NewProxy())
	nap.UseFunc(notFound)

	// assign port number
	port := _config.Port
	if port > 0 && port < 65535 {

	} else {
		port = 8080
	}

	// admin api
	adminNap := napnap.New()
	adminNap.Use(napnap.NewHealth())
	adminNap.Use(newApplicationMiddleware())

	// verify all request which send to admin api and ensure the caller has valid admin token.
	adminRouter := napnap.NewRouter()
	adminRouter.All("/v1", authEndpoint)

	// consumer api
	adminRouter.Put("/v1/consumers", upateOrCreateConsumerEndpoint)
	adminRouter.Get("/v1/consumers/count", getConsumerCountEndpoint)
	adminRouter.Get("/v1/consumers/:consumer_id", getConsumerEndpoint)
	adminRouter.Delete("/v1/consumers/:consumer_id", deletedConsumerEndpoint)

	// token api
	adminRouter.Post("/v1/tokens", createTokenEndpoint)

	adminNap.Use(adminRouter)
	adminNap.UseFunc(notFound)

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
