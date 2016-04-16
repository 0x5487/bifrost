package main

import (
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

type Consumer struct {
	ID           string            `json:"id"`
	App          string            `json:"app"`
	Groups       []string          `json:"groups"`
	Username     string            `json:"username"`
	CustomID     string            `json:"custom_id"`
	CustomFields map[string]string `json:"custom_fields"`
	UpdatedAt    time.Time         `json:"updated_at"`
	CreatedAt    time.Time         `json:"created_at"`
}

type ConsumerRepository interface {
	Get(id string) *Consumer
	GetByUsername(app string, username string) *Consumer
	Insert(consumer *Consumer) error
	Update(consumer *Consumer) error
	Delete(id string) error
	Count() int
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
		return AppError{ErrorCode: "INVALID_DATA", Message: "consumer app filed was invalid."}
	}
	consumer.ID = uuid.NewV4().String()
	now := time.Now()
	consumer.CreatedAt = now
	consumer.UpdatedAt = now
	cs.Lock()
	cs.data[consumer.ID] = consumer
	cs.Unlock()
	return nil
}

func (cs *ConsumerMemStore) Update(consumer *Consumer) error {
	if len(consumer.ID) == 0 {
		return AppError{ErrorCode: "INVALID_DATA", Message: "consumer app filed was invalid."}
	}
	now := time.Now()
	consumer.UpdatedAt = now
	cs.Lock()
	cs.data[consumer.ID] = consumer
	cs.Unlock()
	return nil
}

func (cs *ConsumerMemStore) Delete(id string) error {
	cs.Lock()
	delete(cs.data, id)
	cs.Unlock()
	return nil
}

func (cs *ConsumerMemStore) Count() int {
	cs.RLock()
	defer cs.RUnlock()
	return len(cs.data)
}
