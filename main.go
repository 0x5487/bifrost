package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jasonsoft/napnap"
	"gopkg.in/yaml.v2"
)

var (
	_config         Configuration
	_logger         *logger
	_consumerRepo   ConsumerRepository
	_tokenRepo      TokenRepository
	_status         *status
	_loggerMongo    *loggerMongo
	_app            *Application
	_accessLogsChan chan accessLog
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
		_logger.debug("debug mode was enabled")
		_logger.mode = debugLevel
	}

	// set error logger
	if _config.Logs.ErrorLog.Type == "mongodb" && len(_config.Logs.ErrorLog.ConnectionString) > 0 {
		_logger.debug("enable mongodb log")
		_loggerMongo = newloggerMongo()
	}

	// initial consumer and token storage
	if _config.Data.Type == "memory" {
		_consumerRepo = newConsumerMemStore()
		_tokenRepo = newTokenMemStore()
	}
	if _config.Data.Type == "mongodb" {
		_consumerRepo, err = newConsumerMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
		_tokenRepo, err = newTokenMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
	}
	_app = newApplication()
	_logger.debugf("hostname: %v", _app.Hostname)
}

func main() {
	nap := napnap.New()
	nap.ForwardRemoteIpAddress = true
	nap.UseFunc(requestIDMiddleware())

	// set access log
	if len(_config.Logs.AccessLog.Type) > 0 && _config.Logs.AccessLog.Type == "gelf_udp" {
		_accessLogsChan = make(chan accessLog, 100000)
		_logger.debugf("access log were enabled and connection string are %s", _config.Logs.AccessLog.ConnectionString)
		nap.Use(newAccessLogMiddleware())
		go writeAccessLog(_config.Logs.AccessLog.ConnectionString)
		go listQueueCount()
	}

	_status = newStatusMiddleware()
	nap.Use(_status)

	// turn on gzip feature
	gzip := _config.Gzip
	if gzip.Enable {
		_logger.debug("gzip was enabled")
		nap.Use(napnap.NewGzip(napnap.DefaultCompression))
	}

	// turn on health check feature
	nap.Use(napnap.NewHealth())

	// turn on CORS feature
	cors := _config.Cors
	if cors.Enable {
		options := napnap.Options{}
		options.AllowedOrigins = cors.AllowedOrigins
		options.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
		options.AllowedHeaders = []string{"*"}
		_logger.debugf("cors: %v", strings.Join(options.AllowedOrigins[:], ","))
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
	adminNap.UseFunc(auth) // verify all request which send to admin api and ensure the caller has valid admin token.

	adminRouter := napnap.NewRouter()
	adminRouter.Get("/status", getStatus)

	// consumer api
	adminRouter.Get("/v1/consumers/count", getConsumerCountEndpoint)
	adminRouter.Get("/v1/consumers/:consumer_id", getConsumerEndpoint)
	adminRouter.Delete("/v1/consumers/:consumer_id", deletedConsumerEndpoint)
	adminRouter.Put("/v1/consumers", upateOrCreateConsumerEndpoint)

	// token api
	adminRouter.Get("/v1/tokens/:key", getTokenEndpoint)
	adminRouter.Delete("/v1/tokens/:key", deleteTokenEndpoint)
	adminRouter.Get("/v1/tokens", getTokensEndpoint)
	adminRouter.Post("/v1/tokens", createTokenEndpoint)
	adminRouter.Put("/v1/tokens", updateTokensEndpoint)
	adminRouter.Delete("/v1/tokens", deleteTokensEndpoint)

	adminNap.Use(adminRouter)
	adminNap.UseFunc(notFound)

	// run two http servers on different ports
	// one is for bifrost service and another is for admin api
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		// http server for admin api
		err := adminNap.Run(":8001")
		if err != nil {
			log.Fatal(err)
		}
		wg.Done()
	}()
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
