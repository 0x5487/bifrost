package main

import (
	"strings"
	"sync"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/satori/go.uuid"
)

type Token struct {
	Key         string    `json:"key" bson:"_id"`
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
	Get(key string) (*Token, error)
	GetByConsumerID(consumerID string) ([]*Token, error)
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

func (ts *TokenMemStore) Get(key string) (*Token, error) {
	ts.RLock()
	defer ts.RUnlock()
	result := ts.data[key]
	return result, nil
}

func (ts *TokenMemStore) GetByConsumerID(consumerID string) ([]*Token, error) {
	var result []*Token
	ts.RLock()
	defer ts.RUnlock()
	for _, token := range ts.data {
		if token.ConsumerID == consumerID {
			result = append(result, token)
		}
	}
	return result, nil
}

func (ts *TokenMemStore) Insert(token *Token) error {
	ts.Lock()
	defer ts.Unlock()
	oldToken := ts.data[token.Key]
	if oldToken != nil {
		return AppError{ErrorCode: "invalid_data", Message: "The token key already exits."}
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
		return AppError{ErrorCode: "invalid_data", Message: "The token was not found."}
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

func newTokenMongo(connectionString string) (*tokenMongo, error) {
	session, err := mgo.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("tokens")

	// create index
	consumerIdx := mgo.Index{
		Name:       "token_consumer_idx",
		Key:        []string{"consumer_id"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(consumerIdx)
	if err != nil {
		return nil, err
	}

	return &tokenMongo{
		connectionString: connectionString,
	}, nil
}

func (tm *tokenMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(tm.connectionString)
}

func (tm *tokenMongo) Get(key string) (*Token, error) {
	session, err := tm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	token := Token{}
	err = c.Find(bson.M{"_id": key}).One(&token)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (tm *tokenMongo) GetByConsumerID(consumerID string) ([]*Token, error) {
	session, err := tm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("tokens")
	tokens := []*Token{}
	err = c.Find(bson.M{"consumer_id": consumerID}).All(&tokens)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return tokens, nil
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
		if strings.HasPrefix(err.Error(), "E11000") {
			return AppError{ErrorCode: "invalid_data", Message: "The token key already exits"}
		}
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
	colQuerier := bson.M{"_id": token.Key}
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
	colQuerier := bson.M{"_id": key}
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
