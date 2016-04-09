package main

import (
	"time"

	"github.com/satori/go.uuid"
)

type TokenRepository interface {
	Get(id string) *Token
	GetByConsumerID(consumerID string) []*Token
	Create(token Token) error
	DeleteByConsumerID(consumerID string) error
	Delete(tokenID string) error
}

type Token struct {
	ConsumerID string    `json:"consumer_id"`
	Key        string    `json:"key"`
	Expiration time.Time `json:"expiration"`
	CreatedAt  time.Time `json:"created_at"`
}

func newToken(consumerID string) Token {
	now := time.Now()
	return Token{
		ConsumerID: consumerID,
		Key:        uuid.NewV4().String(),
		Expiration: now.Add(time.Duration(10) * time.Minute),
		CreatedAt:  now,
	}
}

func (t Token) isValid() bool {
	if time.Now().After(t.Expiration) {
		return false
	}
	return true
}
