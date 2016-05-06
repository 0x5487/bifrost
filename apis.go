package main

import (
	"strings"
	"time"

	"github.com/satori/go.uuid"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Policy struct {
	Allow string `json:"allow,omitempty" bson:"allow,omitempty"`
	Deny  string `json:"deny,omitempty" bson:"deny,omitempty"`
}

type Api struct {
	ID               string    `json:"id" bson:"_id"`
	Name             string    `json:"name" bson:"name"`
	RequestHost      string    `json:"request_host" bson:"request_host"`
	RequestPath      string    `json:"request_path" bson:"request_path"`
	StripRequestPath bool      `json:"strip_request_path" bson:"strip_request_path"`
	TargetURL        string    `json:"target_url" bson:"target_url"`
	Redirect         bool      `json:"redirect" bson:"redirect"`
	Policies         []Policy  `json:"policies" bson:"policies"`
	Weight           int       `json:"weight" bson:"weight"`
	CreatedAt        time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" bson:"updated_at"`
}

func (*Api) isValid() bool {
	return true
}

func (a Api) isAllow(consumer Consumer) bool {
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
}

func (p Policy) isAllowPolicy() bool {
	if len(p.Allow) > 0 {
		return true
	}
	return false
}

func (p Policy) isMatch(kind string, consumer Consumer) bool {
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
}

type ApiRepository interface {
	Get(id string) (*Api, error)
	GetByName(name string) (*Api, error)
	GetAll() ([]*Api, error)
	Insert(api *Api) error
	Update(api *Api) error
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

func (ams *apiMongo) Get(id string) (*Api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	api := Api{}
	err = c.FindId(id).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *apiMongo) GetByName(name string) (*Api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	api := Api{}
	err = c.Find(bson.M{"name": name}).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *apiMongo) GetAll() ([]*Api, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("apis")
	apis := []*Api{}
	err = c.Find(bson.M{}).Sort("-weight", "+created_at").All(&apis)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return apis, nil
}

func (ams *apiMongo) Insert(api *Api) error {
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

func (ams *apiMongo) Update(api *Api) error {
	if len(api.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "id can't be empty or null."}
	}
	now := time.Now()
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
