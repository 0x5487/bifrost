package main

import (
	"encoding/json"
	"strings"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	redis "gopkg.in/redis.v4"
)

type configCORS struct {
	Name           string    `json:"-" bson:"name"`
	AllowedOrigins []string  `json:"allowed_origins"  bson:"allowed_origins"`
	UpdatedAt      time.Time `json:"updated_at" bson:"updated_at"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
}

func newConfigCORS() *configCORS {
	return &configCORS{
		AllowedOrigins: []string{},
	}
}

func (cc *configCORS) verifyOrigin(origin string) bool {
	_logger.debugf("origin: %v", origin)
	if contains(cc.AllowedOrigins, "*") || contains(cc.AllowedOrigins, origin) {
		return true
	}
	return false
}

type CORSRepository interface {
	Get() (*configCORS, error)
	Insert(*configCORS) error
	Update(*configCORS) error
	Delete() error
}

/*********************
	Mongo Database
*********************/

type CORSMongo struct {
	connectionString string
}

func newCORSMongo(connectionString string) (*CORSMongo, error) {
	session, err := mgo.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("configs")

	// create index
	nameIdx := mgo.Index{
		Name:       "config_name_idx",
		Key:        []string{"name"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(nameIdx)
	if err != nil {
		return nil, err
	}

	return &CORSMongo{
		connectionString: connectionString,
	}, nil
}

func (cm *CORSMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(cm.connectionString)
}

func (cm *CORSMongo) Get() (*configCORS, error) {
	session, err := cm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("configs")
	var result configCORS
	err = c.Find(bson.M{"name": "cors"}).One(&result)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (cm *CORSMongo) Insert(source *configCORS) error {
	now := time.Now().UTC()
	source.Name = "cors"
	source.CreatedAt = now
	source.UpdatedAt = now

	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("configs")
	err = c.Insert(source)
	if err != nil {
		if strings.HasPrefix(err.Error(), "E11000") {
			return AppError{ErrorCode: "invalid_input", Message: "The config already exists"}
		}
		return err
	}
	return nil
}

func (cm *CORSMongo) Update(source *configCORS) error {
	now := time.Now().UTC()
	source.UpdatedAt = now

	session, err := cm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("configs")
	colQuerier := bson.M{"name": source.Name}
	err = c.Update(colQuerier, source)
	if err != nil {
		return err
	}
	return nil
}

func (cm *CORSMongo) Delete() error {
	return nil
}

func verifyOrigin(origin string) bool {
	return _cors.verifyOrigin(origin)
}

/*********************
	Redis Database
*********************/

type corsRedis struct {
	client *redis.Client
}

func newCorsRedis(addr string, password string, db int) (*corsRedis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	corsRedis := &corsRedis{
		client: client,
	}

	return corsRedis, nil
}

func (cr *corsRedis) Insert(source *configCORS) error {
	nowUTC := time.Now().UTC()
	source.Name = "cors"
	source.CreatedAt = nowUTC
	source.UpdatedAt = nowUTC

	// insert to config:cors
	val, err := json.Marshal(source)
	if err != nil {
		return err
	}
	key := "config:cors"
	err = cr.client.Set(key, val, 0).Err()
	if err != nil {
		return err
	}

	return nil
}

func (cr *corsRedis) Get() (*configCORS, error) {
	key := "config:cors"
	s, err := cr.client.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var result configCORS
	err = json.Unmarshal([]byte(s), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (cr *corsRedis) Update(source *configCORS) error {
	nowUtc := time.Now().UTC()
	source.UpdatedAt = nowUtc

	// update to config:cors
	val, err := json.Marshal(source)
	if err != nil {
		return err
	}
	key := "config:cors"
	err = cr.client.Set(key, val, 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func (cr *corsRedis) Delete() error {
	return nil
}
