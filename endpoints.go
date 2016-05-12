package main

import (
	"runtime"
	"time"

	"github.com/jasonsoft/napnap"
	"github.com/satori/go.uuid"
)

func upateOrCreateConsumerEndpoint(c *napnap.Context) {
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

	err = _consumerRepo.Delete(consumer.ID)
	panicIf(err)
	c.JSON(204, nil)
}

func getTokenEndpoint(c *napnap.Context) {
	key := c.Param("key")

	if len(key) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "key was not found"})
	}

	token, err := _tokenRepo.Get(key)
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
		if tokens == nil {
			c.JSON(200, tokenCollection{})
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

	if len(target.Key) == 0 {
		target.Key = uuid.NewV4().String()
	}

	now := time.Now().UTC()
	if target.Expiration.IsZero() {
		target.Expiration = now.Add(time.Duration(_config.Token.Timeout) * time.Minute)
	} else {
		if now.After(target.Expiration) {
			panic(AppError{ErrorCode: "invalid_data", Message: "expiration field was invalid."})
		}
	}

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
	key := c.Param("key")

	if len(key) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	token, err := _tokenRepo.Get(key)
	panicIf(err)
	if token == nil {
		panic(AppError{ErrorCode: "not_found", Message: "token was not found"})
	}

	err = _tokenRepo.Delete(key)
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

func createServiceEndpoint(c *napnap.Context) {
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

func getServiceEndpoint(c *napnap.Context) {
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
	if _services == nil {
		c.JSON(200, serviceCollection{})
		return
	}
	result := serviceCollection{
		Count:    len(_services),
		Services: _services,
	}
	c.JSON(200, result)
}

func updateServiceEndpoint(c *napnap.Context) {
	serviceID := c.Param("service_id")
	var target service
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
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
	c.JSON(200, service)
}

func deleteServiceEndpoint(c *napnap.Context) {
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

func registerUpstreamEndpoint(c *napnap.Context) {
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
		if svc.ID == serviceID {
			service = svc
		} else if svc.Name == serviceID {
			service = svc
		}
	}
	if service == nil {
		panic(AppError{ErrorCode: "not_found", Message: "service was not found"})
	}

	service.registerUpstream(&target)
	c.JSON(200, target)
}

func reloadEndpoint(c *napnap.Context) {
	_services = reloadService(_serviceRepo, _services)
	c.SetStatus(204)
}

func getStatus(c *napnap.Context) {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	_status.MemoryAcquired = m.Sys / 1000000
	_status.MemoryUsed = m.Alloc / 1000000
	c.JSON(200, _status)
}
