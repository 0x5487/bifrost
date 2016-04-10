package main

import (
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

type Token struct {
	Key        string    `json:"key"`
	ConsumerID string    `json:"consumer_id"`
	Expiration time.Time `json:"expiration"`
	CreatedAt  time.Time `json:"created_at"`
}

func newToken(consumerID string) *Token {
	now := time.Now()
	return &Token{
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

type TokenRepository interface {
	Get(key string) *Token
	GetByConsumerID(consumerID string) []*Token
	Insert(token *Token) error
	DeleteByConsumerID(consumerID string) error
	Delete(tokenID string) error
}

type TokenMemStore struct {
	sync.RWMutex
	data map[string]*Token
}

func newTokenMemStore() TokenMemStore {
	return TokenMemStore{
		data: map[string]*Token{},
	}
}

func (ts TokenMemStore) Get(key string) *Token {
	ts.RLock()
	defer ts.RLock()
	return ts.data[key]
}

func (ts TokenMemStore) GetByConsumerID(consumerID string) []*Token {
	var result []*Token
	ts.RLock()
	defer ts.RLock()
	for _, token := range ts.data {
		if token.ConsumerID == consumerID {
			result = append(result, token)
		}
	}
	return result
}

func (ts TokenMemStore) Insert(token *Token) error {
	ts.Lock()
	ts.data[token.Key] = token
	ts.Unlock()
	return nil
}

func (ts TokenMemStore) Delete(key string) error {
	ts.Lock()
	delete(ts.data, key)
	ts.Unlock()
	return nil
}

func (ts TokenMemStore) DeleteByConsumerID(consumerID string) error {
	ts.Lock()
	for _, token := range ts.data {
		if token.ConsumerID == consumerID {
			delete(ts.data, token.Key)
		}
	}
	ts.Unlock()
	return nil
}
