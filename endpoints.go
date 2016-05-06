package main

import (
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

	if len(consumerID) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	consumer, err := _consumerRepo.Get(consumerID)
	panicIf(err)
	if consumer == nil {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	c.JSON(200, consumer)
}

func getConsumerCountEndpoint(c *napnap.Context) {
	count, err := _consumerRepo.Count()
	panicIf(err)
	result := ApiCount{
		Count: count,
	}
	c.JSON(200, result)
}

func deletedConsumerEndpoint(c *napnap.Context) {
	consumerID := c.Param("consumer_id")

	if len(consumerID) == 0 {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	consumer, err := _consumerRepo.Get(consumerID)
	panicIf(err)
	if consumer == nil {
		panic(AppError{ErrorCode: "not_found", Message: "consumer was not found"})
	}

	err = _consumerRepo.Delete(consumerID)
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

func getTokensEndpoint(c *napnap.Context) {
	consumerId := c.Query("consumer_id")
	if len(consumerId) > 0 {
		result, err := _tokenRepo.GetByConsumerID(consumerId)
		panicIf(err)
		if result == nil {
			c.JSON(200, []Token{})
			return
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

func createAPIEndpoint(c *napnap.Context) {
	var target Api
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	if len(target.Name) == 0 {
		panic(AppError{ErrorCode: "invalid_data", Message: "name field can't be empty or null."})
	}

	api, err := _apiRepo.GetByName(target.Name)
	panicIf(err)

	if api != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: "api already exists."})
	}

	err = _apiRepo.Insert(&target)
	panicIf(err)

	c.JSON(201, target)
}

func getAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")
	api, err := _apiRepo.Get(apiID)
	panicIf(err)

	if api == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}

	c.JSON(200, api)
}

func listAPIEndpoint(c *napnap.Context) {
	apis, err := _apiRepo.GetAll()
	panicIf(err)

	if apis == nil {
		c.JSON(200, []Api{})
		return
	}
	c.JSON(200, apis)
}

func updateAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")
	api, err := _apiRepo.Get(apiID)
	panicIf(err)

	if api == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}

	var target Api
	err = c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "invalid_data", Message: err.Error()})
	}

	target.ID = apiID
	target.CreatedAt = api.CreatedAt

	err = _apiRepo.Update(&target)
	panicIf(err)

	c.JSON(200, api)
}

func deleteAPIEndpoint(c *napnap.Context) {
	apiID := c.Param("api_id")
	api, err := _apiRepo.Get(apiID)
	panicIf(err)

	if api == nil {
		panic(AppError{ErrorCode: "not_found", Message: "api was not found"})
	}

	err = _apiRepo.Delete(apiID)
	panicIf(err)
	c.SetStatus(204)
}

func reloadEndpoint(c *napnap.Context) {
	_apis = reload()
	c.SetStatus(204)
}

func getStatus(c *napnap.Context) {
	c.JSON(200, _status)
}
