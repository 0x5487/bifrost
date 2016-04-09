package main

import (
	"github.com/jasonsoft/napnap"
	"github.com/satori/go.uuid"
)

func authEndpoint(c *napnap.Context) {

}

func upateOrCreateConsumerEndpoint(c *napnap.Context) {
	var target Consumer
	err := c.BindJSON(&target)
	if err != nil {
		panic(AppError{ErrorCode: "INVALID_INPUT", Message: err.Error()})
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
		err = _consumerRepo.Create(&target)
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

func createTokenEndpoint(c *napnap.Context) {
	var token Token
	err := c.BindJSON(&token)
	if err != nil {
		panic(err)
	}

	if len(token.ConsumerID) == 0 {
		panic(AppError{ErrorCode: "INVALID_DATA", Message: "consumer id was invalid."})
	}

	consumer := _consumerRepo.Get(token.ConsumerID)
	if consumer == nil {
		panic(AppError{ErrorCode: "NOT_FOUND", Message: "consumer was not found"})
	}

	newToken := newToken(token.ConsumerID)
	err = _tokenRepo.Create(newToken)
	if err != nil {
		panic(err)
	}

	c.JSON(201, newToken)
}
