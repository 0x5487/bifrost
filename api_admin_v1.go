package main

import (
	"time"

	"github.com/jasonsoft/napnap"
	"github.com/satori/go.uuid"
)

func authEndpoint(c *napnap.Context) {

}

func upateOrCreateConsumerEndpoint(c *napnap.Context) {
	var target Consumer
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: err.Error()})
	}

	if len(target.Username) == 0 {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: "username is invalid."})
	}

	if len(target.App) == 0 {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: "app is invalid."})
	}

	consumer := _consumerRepo.GetByUsername(target.App, target.Username)

	if consumer == nil {
		// create consumer
		target.ID = uuid.NewV4().String()
		err = _consumerRepo.Insert(&target)
		if err != nil {
			panic(err)
		}
		c.JSON(201, target)
		return
	}

	// update consumer
	target.ID = consumer.ID
	target.CreatedAt = consumer.CreatedAt
	err = _consumerRepo.Update(&target)
	if err != nil {
		panic(err)
	}
	c.JSON(200, target)
}

func getConsumerEndpoint(c *napnap.Context) {
	consumerID := c.Param("consumer_id")

	if len(consumerID) == 0 {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found"})
	}

	consumer := _consumerRepo.Get(consumerID)
	if consumer == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found"})
	}

	c.JSON(200, consumer)
}

func getConsumerCountEndpoint(c *napnap.Context) {
	result := ApiCount{
		Count: _consumerRepo.Count(),
	}
	c.JSON(200, result)
}

func deletedConsumerEndpoint(c *napnap.Context) {
	consumerID := c.Param("consumer_id")

	if len(consumerID) == 0 {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found"})
	}

	consumer := _consumerRepo.Get(consumerID)
	if consumer == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found"})
	}

	err := _consumerRepo.Delete(consumerID)
	if err != nil {
		panic(err)
	}

	c.JSON(204, nil)
}

func getTokenEndpoint(c *napnap.Context) {
	key := c.Param("key")

	if len(key) == 0 {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "key was not found"})
	}

	token := _tokenRepo.Get(key)
	if token == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "token was not found"})
	}

	c.JSON(200, token)
}

func getTokensEndpoint(c *napnap.Context) {
	consumerId := c.Query("consumer_id")

	if len(consumerId) > 0 {
		result := _tokenRepo.GetByConsumerID(consumerId)
		c.JSON(200, result)
		return
	}

	//TODO: find all tokens and pagination
}

func createTokenEndpoint(c *napnap.Context) {
	var target Token
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: err.Error()})
	}

	if len(target.ConsumerID) == 0 {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: "consumer_id field was invalid."})
	}

	consumer := _consumerRepo.Get(target.ConsumerID)
	if consumer == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found."})
	}

	if len(target.Key) == 0 {
		target.Key = uuid.NewV4().String()
	}

	now := time.Now().UTC()
	if target.Expiration.IsZero() {
		target.Expiration = now.Add(time.Duration(_config.Token.Timeout) * time.Minute)
	} else {
		if now.After(target.Expiration) {
			panic(AppError{ErrorCode: "INVALID_DATA", Message: "expiration field was invalid."})
		}
	}

	err = _tokenRepo.Insert(&target)
	if err != nil {
		panic(err)
	}
	c.JSON(201, target)
}

func deleteTokenEndpoint(c *napnap.Context) {
	key := c.Param("key")

	if len(key) == 0 {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "token was not found"})
	}

	token := _tokenRepo.Get(key)
	if token == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "token was not found"})
	}

	err := _tokenRepo.Delete(key)
	if err != nil {
		panic(err)
	}

	c.JSON(204, nil)
}

func getStatus(c *napnap.Context) {
	c.JSON(200, _status)
}
