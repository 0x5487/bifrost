package main

import (
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type policy struct {
	Allow string `json:"allow,omitempty" bson:"allow,omitempty"`
	Deny  string `json:"deny,omitempty" bson:"deny,omitempty"`
}

type apiCollection struct {
	Count int    `json:"count"`
	APIs  []*api `json:"apis"`
}

func newAPICollection() *apiCollection {
	return &apiCollection{
		Count: 0,
		APIs:  []*api{},
	}
}

type api struct {
	sync.RWMutex     `json:"-" bson:"-"`
	ID               string    `json:"id" bson:"_id"`
	Name             string    `json:"name" bson:"name"`
	RequestHost      string    `json:"request_host" bson:"request_host"`
	RequestPath      string    `json:"request_path" bson:"request_path"`
	StripRequestPath bool      `json:"strip_request_path" bson:"strip_request_path"`
	TargetURL        string    `json:"target_url" bson:"target_url"`
	Redirect         bool      `json:"redirect" bson:"redirect"`
	Authorization    bool      `json:"authorization" bson:"authorization"`
	Whitelist        []string  `json:"whitelist" bson:"whitelist"`
	Service          string    `json:"service" bson:"service"`
	Weight           int       `json:"weight" bson:"weight"`
	CreatedAt        time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" bson:"updated_at"`
}

func (*api) isValid() bool {
	return true
}

func (a api) isAllow(consumer Consumer) bool {
	if a.Authorization == true && consumer.isAuthenticated() == false {
		return false
	}
	if len(a.Whitelist) == 0 {
		return true
	}
	if len(consumer.Roles) == 0 {
		return false
	}
	for _, role := range consumer.Roles {
		if contains(a.Whitelist, role) {
			return true
		}
	}
	return false
	/*
		for _, policy := range a.Policies {
			if policy.isAllowPolicy() == false {
				if policy.isMatch("deny", consumer) {
					return false
				}
			} else {
				if policy.isMatch("allow", consumer) {
					return true
				}
			}
		}
		// if there isn't any policies, return true
		return true
	*/
}

/*
func (p policy) isAllowPolicy() bool {
	if len(p.Allow) > 0 {
		return true
	}
	return false
}

func (p policy) isMatch(kind string, consumer Consumer) bool {
	var rule string
	if kind == "deny" {
		rule = strings.ToLower(p.Deny)
	}

	if kind == "allow" {
		rule = strings.ToLower(p.Allow)
	}

	if rule == "all" {
		return true
	}
	terms := strings.Split(rule, ":")
	if terms[0] == "g" {
		for _, group := range consumer.Groups {
			if group == terms[1] {
				return true
			}
		}
	}
	return false
}*/

type APIRepository interface {
	Get(id string) (*api, error)
	GetByName(name string) (*api, error)
	GetAll() ([]*api, error)
	Insert(api *api) error
	Update(api *api) error
	Delete(id string) error
}

type apiMongo struct {
	connectionString string
}

func newAPIMongo(connectionString string) (*apiMongo, error) {
	session, err := mgo.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("apis")

	// create index
	nameIdx := mgo.Index{
		Name:       "api_name_idx",
		Key:        []string{"name"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(nameIdx)
	if err != nil {
		return nil, err
	}

	weightIdx := mgo.Index{
		Name:       "api_weight_idx",
		Key:        []string{"-weight", "created_at"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(weightIdx)
	if err != nil {
		return nil, err
	}

	return &apiMongo{
		connectionString: connectionString,
	}, nil
}

func (ams *apiMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(ams.connectionString)
}

func (ams *apiMongo) Get(id string) (*api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	api := api{}
	err = c.FindId(id).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *apiMongo) GetByName(name string) (*api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	api := api{}
	err = c.Find(bson.M{"name": name}).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *apiMongo) GetAll() ([]*api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	apis := []*api{}
	err = c.Find(bson.M{}).Sort("-weight", "+created_at").All(&apis)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return apis, nil
}

func (ams *apiMongo) Insert(api *api) error {
	session, err := ams.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	api.ID = uuid.NewV4().String()
	now := time.Now().UTC()
	api.CreatedAt = now
	api.UpdatedAt = now
	err = c.Insert(api)

	if err != nil {
		if strings.HasPrefix(err.Error(), "E11000") {
			return AppError{ErrorCode: "invalid_data", Message: "The api already exits"}
		}
		return err
	}
	return nil
}

func (ams *apiMongo) Update(api *api) error {
	if len(api.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "id can't be empty or null."}
	}
	now := time.Now().UTC()
	api.UpdatedAt = now

	session, err := ams.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	colQuerier := bson.M{"_id": api.ID}
	err = c.Update(colQuerier, api)
	if err != nil {
		return err
	}
	return nil
}

func (ams *apiMongo) Delete(id string) error {
	session, err := ams.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	colQuerier := bson.M{"_id": id}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}
