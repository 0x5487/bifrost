package main

import (
	"flag"
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
	_status       *status
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

	// parse yaml
	_config = newConfiguration()
	err = yaml.Unmarshal(file, &_config)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	apis := _config.Apis
	if len(apis) == 0 {
		log.Fatalf("config error: no api entries were found.")
	}

	err = _config.isValid()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// setup logger
	_logger = newLog()
	if _config.Debug {
		_logger.mode = Debug
	}

	_consumerRepo = newConsumerMemStore()
	_tokenRepo = newTokenMemStore()
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
	_status = newStatusMiddleware()
	nap.Use(_status)

	// turn on gzip feature
	gzip := _config.Gzip
	if gzip.Enable {
		nap.Use(napnap.NewGzip(napnap.DefaultCompression))
	}

	// turn on health check feature
	nap.Use(napnap.NewHealth())

	if _config.ForwardRequestID {
		nap.UseFunc(requestIDMiddleware())
	}

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

	// admin api
	adminNap := napnap.New()
	adminNap.Use(napnap.NewHealth())
	adminNap.Use(newApplicationMiddleware())
	adminNap.UseFunc(requestIDMiddleware())

	// verify all request which send to admin api and ensure the caller has valid admin token.
	adminRouter := napnap.NewRouter()
	adminRouter.All("/", authEndpoint)
	adminRouter.Get("/status", getStatus)

	// consumer api
	adminRouter.Put("/v1/consumers", upateOrCreateConsumerEndpoint)
	adminRouter.Get("/v1/consumers/count", getConsumerCountEndpoint)
	adminRouter.Get("/v1/consumers/:consumer_id", getConsumerEndpoint)
	adminRouter.Delete("/v1/consumers/:consumer_id", deletedConsumerEndpoint)

	// token api
	adminRouter.Get("/v1/tokens/:key", getTokenEndpoint)
	adminRouter.Get("/v1/tokens", getTokensEndpoint)
	adminRouter.Delete("/v1/tokens/:key", deleteTokenEndpoint)
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
		err := nap.RunAll(_config.Binds)
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()

	wg.Wait()
}
