package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	redis "gopkg.in/redis.v4"
)

type consumerCollection struct {
	Consumers []*Consumer `json:"consumers"`
}

type Consumer struct {
	ID           string            `json:"id" bson:"_id"`
	App          string            `json:"app" bson:"app"`
	Roles        []string          `json:"roles" bson:"roles"`
	Username     string            `json:"username" bson:"username"`
	CustomID     string            `json:"custom_id" bson:"custom_id"`
	CustomFields map[string]string `json:"custom_fields" bson:"custom_fields"`
	UpdatedAt    time.Time         `json:"updated_at" bson:"updated_at"`
	CreatedAt    time.Time         `json:"created_at" bson:"created_at"`
}

func (c *Consumer) isAuthenticated() bool {
	if len(c.ID) > 0 {
		return true
	}
	return false
}

type ConsumerRepository interface {
	Get(id string) (*Consumer, error)
	GetByUsername(app string, username string) (*Consumer, error)
	Insert(consumer *Consumer) error
	Update(consumer *Consumer) error
	Delete(consumer *Consumer) error
	Count(app string) (int, error)
}

type ConsumerMemStore struct {
	sync.RWMutex
	data map[string]*Consumer
}

func newConsumerMemStore() *ConsumerMemStore {
	return &ConsumerMemStore{
		data: map[string]*Consumer{},
	}
}

func (cs *ConsumerMemStore) Get(id string) (*Consumer, error) {
	cs.RLock()
	defer cs.RUnlock()
	result := cs.data[id]
	return result, nil
}

func (cs *ConsumerMemStore) GetByUsername(app string, username string) (*Consumer, error) {
	cs.RLock()
	defer cs.RUnlock()
	var result *Consumer
	for _, consumer := range cs.data {
		if consumer.App == app && consumer.Username == username {
			result = consumer
		}
	}
	return result, nil
}

func (cs *ConsumerMemStore) Insert(consumer *Consumer) error {
	if len(consumer.App) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "app field was invalid."}
	}
	consumer.ID = uuid.NewV4().String()
	now := time.Now().UTC()
	consumer.CreatedAt = now
	consumer.UpdatedAt = now
	cs.Lock()
	defer cs.Unlock()
	cs.data[consumer.ID] = consumer
	return nil
}

func (cs *ConsumerMemStore) Update(consumer *Consumer) error {
	if len(consumer.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "consumer id was invalid."}
	}
	now := time.Now()
	consumer.UpdatedAt = now
	cs.Lock()
	defer cs.Unlock()
	cs.data[consumer.ID] = consumer
	return nil
}

func (cs *ConsumerMemStore) Delete(consumer *Consumer) error {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.data, consumer.ID)
	return nil
}

func (cs *ConsumerMemStore) Count(app string) (int, error) {
	cs.RLock()
	defer cs.RUnlock()
	return len(cs.data), nil
}

/*********************
	Mongo Database
*********************/

type consumerMongo struct {
	connectionString string
}

func newConsumerMongo(connectionString string) (*consumerMongo, error) {
	session, err := mgo.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("consumers")

	// create index
	appUsernameIdx := mgo.Index{
		Name:       "consumer_app_username_idx",
		Key:        []string{"app", "username"},
		Unique:     true,
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(appUsernameIdx)
	if err != nil {
		return nil, err
	}

	return &consumerMongo{
		connectionString: connectionString,
	}, nil
}

func (cm *consumerMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(cm.connectionString)
}

func (cm *consumerMongo) Get(id string) (*Consumer, error) {
	session, err := cm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	consumer := Consumer{}
	err = c.Find(bson.M{"_id": id}).One(&consumer)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &consumer, nil
}

func (cm *consumerMongo) GetByUsername(app string, username string) (*Consumer, error) {
	session, err := cm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	consumer := Consumer{}
	err = c.Find(bson.M{"app": app, "username": username}).One(&consumer)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &consumer, nil
}

func (cm *consumerMongo) Insert(consumer *Consumer) error {
	if len(consumer.App) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "app field was invalid."}
	}
	consumer.ID = uuid.NewV4().String()
	now := time.Now().UTC()
	consumer.CreatedAt = now
	consumer.UpdatedAt = now

	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	err = c.Insert(consumer)
	if err != nil {
		if strings.HasPrefix(err.Error(), "E11000") {
			return AppError{ErrorCode: "invalid_data", Message: "The consumer already exists"}
		}
		return err
	}
	return nil
}

func (cm *consumerMongo) Update(consumer *Consumer) error {
	if len(consumer.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "consumer id was invalid."}
	}
	now := time.Now().UTC()
	consumer.UpdatedAt = now

	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	colQuerier := bson.M{"_id": consumer.ID}
	err = c.Update(colQuerier, consumer)
	if err != nil {
		return err
	}
	return nil
}

func (cm *consumerMongo) Delete(consumer *Consumer) error {
	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	colQuerier := bson.M{"_id": consumer.ID}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}

func (cm *consumerMongo) Count(app string) (int, error) {
	session, err := cm.newSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	count, err := c.Find(bson.M{"app": app}).Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

/*********************
	Redis Database
*********************/

type consumerRedis struct {
	client *redis.Client
}

func newConsumerRedis(addr string, password string, db int) (*consumerRedis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	consumerRedis := &consumerRedis{
		client: client,
	}

	return consumerRedis, nil
}

func (source *consumerRedis) Get(id string) (*Consumer, error) {
	key := "consumer:id:" + id
	s, err := source.client.Get(key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}
		panicIf(err)
	}

	var consumer Consumer
	err = json.Unmarshal([]byte(s), &consumer)
	panicIf(err)

	return &consumer, nil
}

func (source *consumerRedis) GetByUsername(app string, username string) (*Consumer, error) {
	key := "consumer:" + app + ":username:" + username
	consumerID, err := source.client.Get(key).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}
		panicIf(err)
	}

	var consumer *Consumer
	consumer, err = source.Get(consumerID)
	panicIf(err)

	return consumer, nil
}

func (source *consumerRedis) Insert(consumer *Consumer) error {
	if len(consumer.App) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "app field was invalid."}
	}
	consumer.ID = uuid.NewV4().String()
	now := time.Now().UTC()
	consumer.CreatedAt = now
	consumer.UpdatedAt = now

	// insert to consumer:id
	val, err := json.Marshal(consumer)
	panicIf(err)
	key := "consumer:id:" + consumer.ID
	err = source.client.Set(key, val, 0).Err()
	panicIf(err)

	// insert to consumer:app:username
	key = "consumer:" + consumer.App + ":username:" + consumer.Username
	err = source.client.Set(key, consumer.ID, 0).Err()
	panicIf(err)

	return nil
}

func (source *consumerRedis) Update(consumer *Consumer) error {
	if len(consumer.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "consumer id was invalid."}
	}
	now := time.Now().UTC()
	consumer.UpdatedAt = now

	val, err := json.Marshal(consumer)
	panicIf(err)

	key := "consumer:id:" + consumer.ID
	err = source.client.Set(key, val, 0).Err()
	panicIf(err)
	return nil
}

func (source *consumerRedis) Delete(consumer *Consumer) error {
	// delete consumer:id
	key := "consumer:id:" + consumer.ID
	err := source.client.Del(key).Err()
	panicIf(err)

	// delete consumer:username
	key = "consumer:" + consumer.App + ":username:" + consumer.Username
	err = source.client.Del(key).Err()
	panicIf(err)
	return nil
}

func (source *consumerRedis) Count(app string) (int, error) {
	// TODO: need to implement
	return 0, nil
}
