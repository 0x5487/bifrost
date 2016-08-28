package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jasonsoft/napnap"
	"gopkg.in/yaml.v2"
)

var (
	_app          *application
	_httpClient   *http.Client
	_config       Configuration
	_logger       *logger
	_consumerRepo ConsumerRepository
	_tokenRepo    TokenRepository
	_apiRepo      APIRepository
	_corsRepo     CORSRepository
	_serviceRepo  ServiceRepository
	_status       *status
	_apis         []*api
	_cors         *configCORS
	_services     []*service
	_messageChan  chan *gelfMessage
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
		log.Fatal(err)
	}

	// setup logger
	_logger = newLog()
	if _config.Debug {
		_logger.mode = debugLevel
		_logger.info("debug mode was enabled")
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
		_apiRepo, err = newAPIMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
		_serviceRepo, err = newServiceMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
		_corsRepo, err = newCORSMongo(_config.Data.ConnectionString)
		if err != nil {
			panic(err)
		}
	}
	if _config.Data.Type == "redis" {
		db, err := strconv.Atoi(_config.Data.DB)
		_apiRepo, err = newAPIRedis(_config.Data.Address, _config.Data.Password, db)
		if err != nil {
			panic(err)
		}
		_serviceRepo, err = newServiceRedis(_config.Data.Address, _config.Data.Password, db)
		if err != nil {
			panic(err)
		}
		_consumerRepo, err = newConsumerRedis(_config.Data.Address, _config.Data.Password, db)
		if err != nil {
			panic(err)
		}
		_tokenRepo, err = newTokenRedis(_config.Data.Address, _config.Data.Password, db)
		if err != nil {
			panic(err)
		}
	}

	_app = newApplication()
	_logger.infof("hostname: %v", _app.hostname)

	// load api
	_apis, err = _apiRepo.GetAll()
	panicIf(err)
	_services, err = _serviceRepo.GetAll()
	panicIf(err)
}

func main() {
	nap := napnap.New()
	nap.ForwardRemoteIpAddress = true
	nap.UseFunc(requestIDMiddleware())

	// set logs
	if _config.Logs.Target.Type == "gelf" && len(_config.Logs.Target.ConnectionString) > 0 {
		_messageChan = make(chan *gelfMessage, 30000) // TODO: allow user to set the value via config file
		go writeAccessLog(_config.Logs.Target.ConnectionString)
		_logger.infof("log was enabled and connection string is %s", _config.Logs.Target.ConnectionString)

		// set access log
		if _config.Logs.AccessLog {
			nap.Use(newAccessLogMiddleware())
			_logger.info("access log was enabled")
		}
		// set application log
		if _config.Logs.ApplicationLog {
			nap.Use(newApplicationLogMiddleware(true))
			_logger.info("application log was enabled")
		} else {
			nap.Use(newApplicationLogMiddleware(false))
		}
	}

	// set custom errors
	if _config.CustomErrors {
		nap.Use(newCustomErrorsMiddleware())
	}

	nap.Use(_app)

	// turn on gzip feature
	gzip := _config.Gzip
	if gzip.Enable {
		_logger.info("gzip was enabled")
		nap.Use(napnap.NewGzip(napnap.DefaultCompression))
	}

	// turn on health check feature
	nap.Use(napnap.NewHealth())

	// turn on CORS feature
	cors := _config.Cors
	if cors.Enable {
		options := napnap.Options{}
		var err error
		_cors, err = _corsRepo.Get()
		panicIf(err)
		if _cors == nil {
			_cors = newConfigCORS()
		}
		options.AllowOriginFunc = verifyOrigin
		options.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE"}
		options.AllowedHeaders = []string{"*"}
		nap.Use(napnap.NewCors(options))
		_logger.infof("cors was enabled: %v", strings.Join(_cors.AllowedOrigins[:], ","))
	}

	nap.UseFunc(identity)
	nap.Use(newProxy())
	nap.UseFunc(notFound)

	// admin endpoints
	adminNap := napnap.New()
	adminNap.Use(napnap.NewHealth())
	adminNap.Use(newApplicationLogMiddleware(false))
	adminNap.UseFunc(requestIDMiddleware())
	adminNap.UseFunc(auth) // verify all request which send to admin api and ensure the caller has valid admin token.

	adminRouter := napnap.NewRouter()
	adminRouter.Get("/status", getStatus)

	// consumer endpoints
	adminRouter.Get("/v1/consumers/count", getConsumerCountEndpoint)
	adminRouter.Get("/v1/consumers/:consumer_id", getConsumerEndpoint)
	adminRouter.Delete("/v1/consumers/:consumer_id", deletedConsumerEndpoint)
	adminRouter.Put("/v1/consumers", createOrupateConsumerEndpoint)

	// token endpoints
	//adminRouter.Put("/v1/tokens/:key/expire", expireTokenEndpoint) //deprecated
	adminRouter.Get("/v1/tokens/:id", getTokenEndpoint)
	adminRouter.Delete("/v1/tokens/:id", deleteTokenEndpoint)
	adminRouter.Get("/v1/tokens", listTokensEndpoint)
	adminRouter.Post("/v1/tokens", createTokenEndpoint)
	adminRouter.Put("/v1/tokens", updateTokensEndpoint)
	adminRouter.Delete("/v1/tokens", deleteTokensEndpoint)

	// api endpoints
	adminRouter.Post("/v1/apis/switch", switchAPISource)
	adminRouter.Put("/v1/apis/reload", reloadAPIEndpoint)
	adminRouter.Get("/v1/apis/:api_id", getAPIEndpoint)
	adminRouter.Delete("/v1/apis/:api_id", deleteAPIEndpoint)
	adminRouter.Put("/v1/apis/:api_id", updateAPIEndpoint)
	adminRouter.Get("/v1/apis", listAPIEndpoint)
	adminRouter.Post("/v1/apis", createAPIEndpoint)

	// upstream endpoints
	adminRouter.Delete("/v1/services/:service_id/upstreams/:upstream_id", unregisterServiceUpstreamEndpoint)
	adminRouter.Put("/v1/services/:service_id/upstreams", registerServiceUpstreamEndpoint)

	// service endpoints
	adminRouter.Put("/v1/services/reload", reloadServiceEndpoint)
	adminRouter.Delete("/v1/services/:service_id", deleteServicesEndpoint)
	adminRouter.Put("/v1/services/:service_id", updateServicesEndpoint)
	adminRouter.Get("/v1/services/:service_id", getServicesEndpoint)
	adminRouter.Post("/v1/services", createServicesEndpoint)
	adminRouter.Get("/v1/services", listServicesEndpoint)

	// config endpoints
	adminRouter.Put("/v1/configs/cors/reload", reloadCORSEndpoint)
	adminRouter.Get("/v1/configs/cors", getCORSEndpoint)
	adminRouter.Put("/v1/configs/cors", createOrUpdateCORSEndpoint)

	adminNap.Use(adminRouter)
	adminNap.UseFunc(notFound)

	// run two http servers on different ports
	// one is for bifrost service and another is for admin api
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		// http server for admin api
		err := adminNap.Run(":10081")
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
