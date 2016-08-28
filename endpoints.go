package main

import (
	"runtime"
	"strings"
	"time"

	"github.com/jasonsoft/napnap"
	"github.com/satori/go.uuid"
)

func createOrupateConsumerEndpoint(c *napnap.Context) {
	var target Consumer
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	if len(target.Username) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "username field is invalid."})
	}

	if len(target.App) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "app field is invalid."})
	}

	consumer, err := _consumerRepo.GetByUsername(target.App, target.Username)
	panicIf(err)

	if consumer == nil {
		// create consumer
		target.ID = uuid.NewV4().String()
		err = _consumerRepo.Insert(&target)
		panicIf(err)
		c.JSON(201, target)
		return
	}

	// update consumer
	target.ID = consumer.ID
	target.CreatedAt = consumer.CreatedAt
	err = _consumerRepo.Update(&target)
	panicIf(err)
	c.JSON(200, target)
}

func getConsumerEndpoint(c *napnap.Context) {
	consumerID := c.Param("consumer_id")
	app := c.Query("app")
	if len(app) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "app field was missing or empty"})
	}
	consumer, err := _consumerRepo.Get(consumerID)
	panicIf(err)
	if consumer == nil {
		consumer, err = _consumerRepo.GetByUsername(app, consumerID)
		panicIf(err)
	}
	if consumer == nil {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	c.JSON(200, consumer)
}

func getConsumerCountEndpoint(c *napnap.Context) {
	// redis provider doesn't support this feature.
	if _config.Data.Type == "redis" {
		c.SetStatus(501)
		return
	}

	app := c.Query("app")
	if len(app) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "app field was missing or empty"})
	}
	count, err := _consumerRepo.Count(app)
	panicIf(err)
	result := ApiCount{
		Count: count,
	}
	c.JSON(200, result)
}

func deletedConsumerEndpoint(c *napnap.Context) {
	consumerID := c.Param("consumer_id")
	app := c.Query("app")
	if len(app) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "app field was missing or empty"})
	}

	consumer, err := _consumerRepo.Get(consumerID)
	panicIf(err)
	if consumer == nil {
		consumer, err = _consumerRepo.GetByUsername(app, consumerID)
		panicIf(err)
	}
	if consumer == nil {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	err = _consumerRepo.Delete(consumer)
	panicIf(err)
	c.JSON(204, nil)
}

func getTokenEndpoint(c *napnap.Context) {
	id := c.Param("id")

	if len(id) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "key was not found"})
	}

	token, err := _tokenRepo.Get(id)
	panicIf(err)
	if token == nil {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	c.JSON(200, token)
}

func listTokensEndpoint(c *napnap.Context) {
	consumerId := c.Query("consumer_id")
	if len(consumerId) > 0 {
		tokens, err := _tokenRepo.GetByConsumerID(consumerId)
		panicIf(err)
		if len(tokens) == 0 {
			_tokenRepo.DeleteByConsumerID(consumerId) // for redis
			c.JSON(200, newTokenCollection())
			return
		}
		result := tokenCollection{
			Count:  len(tokens),
			Tokens: tokens,
		}
		c.JSON(200, result)
		return
	}
	//TODO: find all tokens and pagination
	c.SetStatus(501)
	return
}

func createTokenEndpoint(c *napnap.Context) {
	var target Token
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	if len(target.ConsumerID) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "consumer_id field was invalid."})
	}

	consumer, err := _consumerRepo.Get(target.ConsumerID)
	panicIf(err)
	if consumer == nil {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found."})
	}

	if len(target.ID) == 0 {
		target.ID = uuid.NewV4().String()
	}

	now := time.Now().UTC()
	if target.ExpiresIn > 0 {
		target.Expiration = now.Add(time.Duration(target.ExpiresIn) * time.Second)
	} else {
		target.Expiration = now.Add(time.Duration(_config.Token.Timeout) * time.Second)
	}
	target.ExpiresIn = int64(target.Expiration.Sub(now).Seconds())

	err = _tokenRepo.Insert(&target)
	panicIf(err)
	c.JSON(201, target)
}

func updateTokensEndpoint(c *napnap.Context) {
	var tokens []Token
	err := c.BindJSON(&tokens)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(tokens) == 0 {
		c.SetStatus(204)
		return
	}
	for _, token := range tokens {
		_tokenRepo.Update(&token)
	}
	c.SetStatus(204)
}

func deleteTokenEndpoint(c *napnap.Context) {
	id := c.Param("id")

	if len(id) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	token, err := _tokenRepo.Get(id)
	panicIf(err)
	if token == nil {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	err = _tokenRepo.Delete(id)
	panicIf(err)

	c.SetStatus(204)
}

func expireTokenEndpoint(c *napnap.Context) {
	key := c.Param("key")

	if len(key) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	token, err := _tokenRepo.Get(key)
	panicIf(err)
	if token == nil {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	token.Expiration = time.Now().UTC()
	err = _tokenRepo.Update(token)
	panicIf(err)

	c.SetStatus(204)
}

func deleteTokensEndpoint(c *napnap.Context) {
	consumerId := c.Query("consumer_id")
	var tokens []*Token
	var err error

	if len(consumerId) > 0 {
		// get all tokens by consumer id
		tokens, err = _tokenRepo.GetByConsumerID(consumerId)
		panicIf(err)
	}

	if len(tokens) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "tokens were not found"})
	}

	// delete all token by consumer id
	_tokenRepo.DeleteByConsumerID(consumerId)
	c.SetStatus(204)
}

func createAPIEndpoint(c *napnap.Context) {
	var target api
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(target.Name) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "name field can't be empty or null"})
	}
	/*
		api, err := _apiRepo.GetByName(target.Name)
		panicIf(err)
		if api != nil {
			panic(AppError{ErrorCode: "invalid_data", Message: "name already exists"})
		}
	*/
	if target.Whitelist == nil {
		target.Whitelist = []string{}
	}
	err = _apiRepo.Insert(&target)
	panicIf(err)
	c.JSON(201, target)
}

func getAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")

	var result *api
	for _, api := range _apis {
		if api.ID == apiID {
			result = api
			break
		}
		if api.Name == apiID {
			result = api
			break
		}
	}

	if result == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}

	c.JSON(200, result)
}

func listAPIEndpoint(c *napnap.Context) {
	mode := c.Query("mode")
	result := newAPICollection()
	if mode == "preview" {
		apis, err := _apiRepo.GetAll()
		panicIf(err)
		if len(apis) > 0 {
			result = &apiCollection{
				Count: len(apis),
				APIs:  apis,
			}
			c.JSON(200, result)
			return
		}
	}

	if len(_apis) > 0 {
		result = &apiCollection{
			Count: len(_apis),
			APIs:  _apis,
		}
	}
	c.JSON(200, result)
}

func updateAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")
	var target api
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(target.Name) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "name field can't be empty"})
	}

	api, err := _apiRepo.Get(apiID)
	panicIf(err)
	if api == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}

	target.ID = api.ID
	if target.Whitelist == nil {
		target.Whitelist = []string{}
	}
	target.CreatedAt = api.CreatedAt
	err = _apiRepo.Update(&target)
	panicIf(err)
	c.JSON(200, target)
}

func deleteAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")
	api, err := _apiRepo.Get(apiID)
	panicIf(err)
	if api == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}
	err = _apiRepo.Delete(api.ID)
	panicIf(err)
	c.SetStatus(204)
}

func switchAPISource(c *napnap.Context) {
	var target apiSwitch
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(target.From) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "from field can't be empty"})
	}
	if len(target.To) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "to field can't be empty"})
	}

	apiFrom, err := _apiRepo.Get(target.From)
	panicIf(err)
	if apiFrom == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api of from field was not found"})
	}

	apiTo, err := _apiRepo.Get(target.To)
	panicIf(err)
	if apiTo == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api of to field was not found"})
	}

	// update
	apiFrom.switchSource(apiTo)
	err = _apiRepo.Update(apiFrom)
	panicIf(err)
	err = _apiRepo.Update(apiTo)
	panicIf(err)

	// reload api
	_apis, err = _apiRepo.GetAll()
	panicIf(err)
	c.SetStatus(200)
}

func reloadAPIEndpoint(c *napnap.Context) {
	var err error
	_apis, err = _apiRepo.GetAll()
	panicIf(err)
	c.SetStatus(204)
}

func createOrUpdateCORSEndpoint(c *napnap.Context) {
	var target configCORS
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	cors, err := _corsRepo.Get()
	panicIf(err)

	if cors == nil {
		// create configCORS
		target.Name = "cors"
		err = _corsRepo.Insert(&target)
		panicIf(err)
		c.JSON(201, target)
		return
	}

	// update configCORS
	cors.AllowedOrigins = target.AllowedOrigins
	err = _corsRepo.Update(cors)
	panicIf(err)
	c.JSON(200, cors)

}

func getCORSEndpoint(c *napnap.Context) {
	mode := strings.ToLower(c.Query("mode"))

	// preview mode
	if mode == "preview" {
		cors, err := _corsRepo.Get()
		panicIf(err)
		if cors == nil {
			c.SetStatus(404)
		}
		c.JSON(200, cors)
		return
	}

	// nornal mode
	c.JSON(200, _cors)
}

func reloadCORSEndpoint(c *napnap.Context) {
	var err error
	_cors, err = _corsRepo.Get()
	panicIf(err)
	c.SetStatus(204)
}

func createServicesEndpoint(c *napnap.Context) {
	var target service
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(target.Name) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "name field can't be empty or null"})
	}
	service, err := _serviceRepo.GetByName(target.Name)
	panicIf(err)
	if service != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: "name already exists"})
	}

	target.Upstreams = []*upstream{}

	err = _serviceRepo.Insert(&target)
	panicIf(err)

	c.JSON(201, target)
}

func getServicesEndpoint(c *napnap.Context) {
	serviceID := c.Param("service_id")

	var result *service
	for _, svc := range _services {
		if svc.ID == serviceID {
			result = svc
			break
		}
		if svc.Name == serviceID {
			result = svc
			break
		}
	}

	if result == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}

	c.JSON(200, result)
}

func listServicesEndpoint(c *napnap.Context) {
	mode := c.Query("mode")
	result := newServiceCollection()
	if mode == "preview" {
		services, err := _serviceRepo.GetAll()
		panicIf(err)
		if len(services) > 0 {
			result = &serviceCollection{
				Count:    len(services),
				Services: services,
			}
			c.JSON(200, result)
			return
		}
	}
	if len(_services) > 0 {
		result = &serviceCollection{
			Count:    len(_services),
			Services: _services,
		}
	}
	c.JSON(200, result)
}

func updateServicesEndpoint(c *napnap.Context) {
	serviceID := c.Param("service_id")
	var target service
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}
	if len(target.Name) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "name field can't be empty"})
	}

	service, err := _serviceRepo.Get(serviceID)
	panicIf(err)
	if service == nil {
		service, err = _serviceRepo.GetByName(serviceID)
	}
	if service == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}

	target.ID = service.ID
	target.Upstreams = service.Upstreams
	target.CreatedAt = service.CreatedAt
	err = _serviceRepo.Update(&target)
	panicIf(err)
	c.JSON(200, target)
}

func deleteServicesEndpoint(c *napnap.Context) {
	serviceID := c.Param("service_id")
	service, err := _serviceRepo.Get(serviceID)
	panicIf(err)
	if service == nil {
		service, err = _serviceRepo.GetByName(serviceID)
	}
	if service == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}
	err = _serviceRepo.Delete(service.ID)
	panicIf(err)
	c.SetStatus(204)
}

func registerServiceUpstreamEndpoint(c *napnap.Context) {
	var target upstream
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	// verify input
	if len(target.TargetURL) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "target_url field was missing or empty"})
	}

	serviceID := c.Param("service_id")
	var service *service
	for _, svc := range _services {
		if svc.ID == serviceID || svc.Name == serviceID {
			service = svc
		}
	}
	if service == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}

	service.registerUpstream(&target)
	c.JSON(200, target)
}

func unregisterServiceUpstreamEndpoint(c *napnap.Context) {
	serviceID := c.Param("service_id")
	var service *service
	for _, svc := range _services {
		if svc.ID == serviceID {
			service = svc
		} else if svc.Name == serviceID {
			service = svc
		}
	}
	if service == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}

	upstreamID := c.Param("upstream_id")
	for _, upS := range service.Upstreams {
		if upS.Name == upstreamID {
			// remove upstream
			service.unregisterUpstream(upS)
			c.SetStatus(204)
			return
		}
	}
	panic(AppError{ErrorCode: "not_found", Message: "upstream was not found"})
}

func reloadServiceEndpoint(c *napnap.Context) {
	services, err := _serviceRepo.GetAll()
	panicIf(err)
	for _, newSvc := range services {
		for _, oldSvc := range _services {
			if newSvc.ID == oldSvc.ID {
				newSvc.Upstreams = oldSvc.Upstreams
			}
		}
	}
	_services = services
	c.SetStatus(204)
}

func getStatus(c *napnap.Context) {
	status := status{}
	status.Hostname = _app.hostname
	status.ServerTime = time.Now().UTC()
	status.NumCPU = runtime.NumCPU()
	status.TotalRequests = _app.totalRequests
	status.NetworkIn = _app.networkIn / 1000000
	status.NetworkOut = _app.networkOut / 1000000
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	status.MemoryAcquired = m.Sys / 1000000
	status.MemoryUsed = m.Alloc / 1000000
	status.StartAt = _app.startAt
	status.Uptime = time.Since(_app.startAt).String()
	c.JSON(200, status)
}
