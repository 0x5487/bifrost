package main

import (
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Consumer struct {
	ID           string            `json:"id" bson:"id"`
	App          string            `json:"app" bson:"app"`
	Groups       []string          `json:"groups" bson:"groups"`
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
	Get(id string) *Consumer
	GetByUsername(app string, username string) *Consumer
	Insert(consumer *Consumer) error
	Update(consumer *Consumer) error
	Delete(id string) error
	Count() (int, error)
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

func (cs *ConsumerMemStore) Get(id string) *Consumer {
	cs.RLock()
	defer cs.RUnlock()
	result := cs.data[id]
	return result
}

func (cs *ConsumerMemStore) GetByUsername(app string, username string) *Consumer {
	cs.RLock()
	defer cs.RUnlock()
	var result *Consumer
	for _, consumer := range cs.data {
		if consumer.App == app && consumer.Username == username {
			result = consumer
		}
	}
	return result
}

func (cs *ConsumerMemStore) Insert(consumer *Consumer) error {
	if len(consumer.App) == 0 {
		return AppError{ErrorCode: "INVALID_DATA", Message: "app filed was invalid."}
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
		return AppError{ErrorCode: "INVALID_DATA", Message: "app filed was invalid."}
	}
	now := time.Now()
	consumer.UpdatedAt = now
	cs.Lock()
	defer cs.Unlock()
	cs.data[consumer.ID] = consumer
	return nil
}

func (cs *ConsumerMemStore) Delete(id string) error {
	cs.Lock()
	defer cs.Unlock()
	delete(cs.data, id)
	return nil
}

func (cs *ConsumerMemStore) Count() (int, error) {
	cs.RLock()
	defer cs.RUnlock()
	return len(cs.data), nil
}

type consumerMongo struct {
	connectionString string
}

func newConsumerMongo(connectionString string) *consumerMongo {
	return &consumerMongo{
		connectionString: connectionString,
	}
}

func (cm *consumerMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(cm.connectionString)
}

func (cm *consumerMongo) Get(id string) *Consumer {
	session, err := cm.newSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	consumer := Consumer{}
	err = c.Find(bson.M{"id": id}).One(&consumer)
	if err != nil {
		if err.Error() == "not found" {
			return nil
		}
		panic(err)
	}
	return &consumer
}

func (cm *consumerMongo) GetByUsername(app string, username string) *Consumer {
	session, err := cm.newSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	consumer := Consumer{}
	err = c.Find(bson.M{"app": app, "username": username}).One(&consumer)
	if err != nil {
		if err.Error() == "not found" {
			return nil
		}
		panic(err)
	}
	return &consumer
}

func (cm *consumerMongo) Insert(consumer *Consumer) error {
	if len(consumer.App) == 0 {
		return AppError{ErrorCode: "INVALID_DATA", Message: "app filed was invalid."}
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
		return err
	}
	return nil
}

func (cm *consumerMongo) Update(consumer *Consumer) error {
	if len(consumer.ID) == 0 {
		return AppError{ErrorCode: "INVALID_DATA", Message: "app filed was invalid."}
	}
	now := time.Now()
	consumer.UpdatedAt = now

	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	colQuerier := bson.M{"id": consumer.ID}
	err = c.Update(colQuerier, consumer)
	if err != nil {
		return err
	}
	return nil
}

func (cm *consumerMongo) Delete(id string) error {
	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	colQuerier := bson.M{"id": id}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}

func (cm *consumerMongo) Count() (int, error) {
	session, err := cm.newSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("consumers")
	count, err := c.Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}
