package main

import (
	"time"

	"github.com/jasonsoft/napnap"
)

type (
	Consumer struct {
		ID        string            `json:"id"`
		CustomID  string            `json:"custom_id"`
		Name      string            `json:"name"`
		Extension map[string]string `json:"ext"`
		Tokens    []Token           `json:"tokens"`
		CreatedAt time.Time         `json:"created_at"`
	}
	Token struct {
		Key       string    `json:"key"`
		CreatedAt time.Time `json:"created_at"`
	}
)

func createConsumerEndpoint(c *napnap.Context) {
	var consumer Consumer
	err := c.BindJSON(&consumer)
	if err != nil {
		panic(err)
	}

}

func getConsumerEndpoint(c *napnap.Context) {

}

func deletedConsumerEndpoint(c *napnap.Context) {

}

func createTokenEndpoint(c *napnap.Context) {

}
