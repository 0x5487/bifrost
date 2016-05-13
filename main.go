package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jasonsoft/napnap"
	"gopkg.in/yaml.v2"
)

var (
	_app            *application
	_httpClient     *http.Client
	_config         Configuration
	_logger         *logger
	_consumerRepo   ConsumerRepository
	_tokenRepo      TokenRepository
	_serviceRepo    ServiceRepository
	_status         *status
	_services       []*service
	_accessLogsChan chan accessLog
	_errorLogsChan  chan applocationLog
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

	_httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
		},
		Timeout: time.Duration(30) * time.Second,
	}

	// parse yaml
	_config = newConfiguration()
	err = yaml.Unmarshal(file, &_config)
	if err != nil {
		log.Fatalf("config error: %v", err)
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

		_serviceRepo, err = newAPIMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
	}

	_app = newApplication()
	_logger.debugf("hostname: %v", _app.hostname)

	// reload
	_services = reloadService(_serviceRepo, _services)
}

func main() {
	nap := napnap.New()
	nap.ForwardRemoteIpAddress = true
	nap.UseFunc(requestIDMiddleware())

	// set access log
	if _config.Logs.AccessLog.Type == "graylog" && len(_config.Logs.AccessLog.ConnectionString) > 0 {
		_accessLogsChan = make(chan accessLog, 20000)
		nap.Use(newAccessLogMiddleware())
		go writeAccessLog(_config.Logs.AccessLog.ConnectionString)
		_logger.debugf("access log were enabled and connection string is %s", _config.Logs.AccessLog.ConnectionString)
	}

	// set custom errors
	if _config.CustomErrors {
		nap.Use(newCustomErrorsMiddleware())
	}

	// set error logger
	nap.Use(newErrorLogMiddleware(true))
	if _config.Logs.ApplicationLog.Type == "graylog" && len(_config.Logs.ApplicationLog.ConnectionString) > 0 {
		_errorLogsChan = make(chan applocationLog, 10000)
		go writeApplicationLog(_config.Logs.ApplicationLog.ConnectionString)
		_logger.debugf("application log were enabled and connection string is %s", _config.Logs.ApplicationLog.ConnectionString)
	}

	nap.Use(_app)

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
		//options.AllowedOrigins = cors.AllowedOrigins
		options.AllowOriginFunc = checkOriginForCORS
		options.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
		options.AllowedHeaders = []string{"*"}
		_logger.debugf("cors was enabled: %v", strings.Join(options.AllowedOrigins[:], ","))
		nap.Use(napnap.NewCors(options))
	}

	nap.UseFunc(identity)
	nap.Use(newProxy())
	nap.UseFunc(notFound)

	// admin endpoints
	adminNap := napnap.New()
	adminNap.Use(napnap.NewHealth())
	adminNap.Use(newErrorLogMiddleware(false))
	adminNap.UseFunc(requestIDMiddleware())
	adminNap.UseFunc(auth) // verify all request which send to admin api and ensure the caller has valid admin token.

	adminRouter := napnap.NewRouter()
	adminRouter.Get("/status", getStatus)
	adminRouter.Put("/reload", reloadEndpoint)

	// consumer endpoints
	adminRouter.Get("/v1/consumers/count", getConsumerCountEndpoint)
	adminRouter.Get("/v1/consumers/:consumer_id", getConsumerEndpoint)
	adminRouter.Delete("/v1/consumers/:consumer_id", deletedConsumerEndpoint)
	adminRouter.Put("/v1/consumers", upateOrCreateConsumerEndpoint)

	// token endpoints
	adminRouter.Put("/v1/tokens/:key/expire", expireTokenEndpoint)
	adminRouter.Get("/v1/tokens/:key", getTokenEndpoint)
	adminRouter.Delete("/v1/tokens/:key", deleteTokenEndpoint)
	adminRouter.Get("/v1/tokens", listTokensEndpoint)
	adminRouter.Post("/v1/tokens", createTokenEndpoint)
	adminRouter.Put("/v1/tokens", updateTokensEndpoint)
	adminRouter.Delete("/v1/tokens", deleteTokensEndpoint)

	// service endpoints
	adminRouter.Get("/v1/services/:service_id", getServiceEndpoint)
	adminRouter.Delete("/v1/services/:service_id", deleteServiceEndpoint)
	adminRouter.Put("/v1/services/:service_id", updateServiceEndpoint)
	adminRouter.Get("/v1/services", listServicesEndpoint)
	adminRouter.Post("/v1/services", createServiceEndpoint)

	// upstream endpoints
	//adminRouter.Delete("/v1/services/:service_id/upstreams/:upstream_id", unRegisterUpstreamEndpoint)
	adminRouter.Put("/v1/services/:service_id/upstreams", registerUpstreamEndpoint)

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
