package main

import (
	"sync"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/satori/go.uuid"
)

type Token struct {
	Key         string    `json:"key" bson:"key"`
	Source      string    `json:"source" bson:"source"`
	ConsumerID  string    `json:"consumer_id" bson:"consumer_id"`
	IPAddresses []string  `json:"ip_addresses" bson:"ip_addresses"`
	Expiration  time.Time `json:"expiration" bson:"expiration"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
}

func newToken(consumerID string) *Token {
	now := time.Now().UTC()
	return &Token{
		ConsumerID: consumerID,
		Key:        uuid.NewV4().String(),
		Expiration: now.Add(time.Duration(_config.Token.Timeout) * time.Minute),
		CreatedAt:  now,
	}
}

func (t *Token) isValid() bool {
	if time.Now().UTC().After(t.Expiration) {
		return false
	}
	return true
}

func (t *Token) renew() {
	t.Expiration = time.Now().UTC().Add(time.Duration(_config.Token.Timeout) * time.Minute)
}

type TokenRepository interface {
	Get(key string) *Token
	GetByConsumerID(consumerID string) []*Token
	Insert(token *Token) error
	Update(token *Token) error
	DeleteByConsumerID(consumerID string) error
	Delete(key string) error
}

type TokenMemStore struct {
	sync.RWMutex
	data map[string]*Token
}

func newTokenMemStore() *TokenMemStore {
	return &TokenMemStore{
		data: map[string]*Token{},
	}
}

func (ts *TokenMemStore) Get(key string) *Token {
	ts.RLock()
	defer ts.RUnlock()
	result := ts.data[key]
	return result
}

func (ts *TokenMemStore) GetByConsumerID(consumerID string) []*Token {
	var result []*Token
	ts.RLock()
	defer ts.RUnlock()
	for _, token := range ts.data {
		if token.ConsumerID == consumerID {
			result = append(result, token)
		}
	}
	return result
}

func (ts *TokenMemStore) Insert(token *Token) error {
	ts.Lock()
	defer ts.Unlock()
	oldToken := ts.data[token.Key]
	if oldToken != nil {
		return AppError{ErrorCode: "INVALID_DATA", Message: "The token key already exits."}
	}
	token.CreatedAt = time.Now().UTC()
	ts.data[token.Key] = token
	return nil
}

func (ts *TokenMemStore) Update(token *Token) error {
	ts.Lock()
	defer ts.Unlock()
	oldToken := ts.data[token.Key]
	if oldToken == nil {
		return AppError{ErrorCode: "INVALID_DATA", Message: "The token was not found."}
	}
	ts.data[token.Key] = token
	return nil
}

func (ts *TokenMemStore) Delete(key string) error {
	ts.Lock()
	defer ts.Unlock()
	delete(ts.data, key)
	return nil
}

func (ts *TokenMemStore) DeleteByConsumerID(consumerID string) error {
	ts.Lock()
	defer ts.Unlock()
	for _, token := range ts.data {
		if token.ConsumerID == consumerID {
			delete(ts.data, token.Key)
		}
	}
	return nil
}

type tokenMongo struct {
	connectionString string
}

func newTokenMongo(connectionString string) *tokenMongo {
	// TODO: create token collection and build index

	return &tokenMongo{
		connectionString: connectionString,
	}
}

func (tm *tokenMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(tm.connectionString)
}

func (tm *tokenMongo) Get(key string) *Token {
	session, err := tm.newSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	token := Token{}
	err = c.Find(bson.M{"key": key}).One(&token)
	if err != nil {
		if err.Error() == "not found" {
			return nil
		}
		panic(err)
	}
	return &token
}

func (tm *tokenMongo) GetByConsumerID(consumerID string) []*Token {
	session, err := tm.newSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	tokens := []*Token{}
	err = c.Find(bson.M{"consumer_id": consumerID}).All(&tokens)
	if err != nil {
		if err.Error() == "not found" {
			return nil
		}
		panic(err)
	}
	return tokens
}

func (tm *tokenMongo) Insert(token *Token) error {
	session, err := tm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	token.CreatedAt = time.Now().UTC()
	err = c.Insert(token)
	if err != nil {
		//TODO: duplicate key
		return err
	}
	return nil
}

func (tm *tokenMongo) Update(token *Token) error {
	session, err := tm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	colQuerier := bson.M{"key": token.Key}
	err = c.Update(colQuerier, token)
	if err != nil {
		return err
	}
	return nil
}

func (tm *tokenMongo) Delete(key string) error {
	session, err := tm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	colQuerier := bson.M{"key": key}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}

func (tm *tokenMongo) DeleteByConsumerID(consumerID string) error {
	session, err := tm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	colQuerier := bson.M{"consumer_id": consumerID}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}
