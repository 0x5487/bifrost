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

type upstream struct {
	count     int
	Name      string    `json:"name" bson:"name"`
	TargetURL string    `json:"target_url" bson:"target_url"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type serviceCollection struct {
	Count    int        `json:"count"`
	Services []*service `json:"services"`
}

type service struct {
	sync.RWMutex
	ID               string      `json:"id" bson:"_id"`
	Name             string      `json:"name" bson:"name"`
	RequestHost      string      `json:"request_host" bson:"request_host"`
	RequestPath      string      `json:"request_path" bson:"request_path"`
	StripRequestPath bool        `json:"strip_request_path" bson:"strip_request_path"`
	Upstreams        []*upstream `json:"upstreams" bson:"upstreams"`
	Redirect         bool        `json:"redirect" bson:"redirect"`
	Policies         []policy    `json:"policies" bson:"policies"`
	Weight           int         `json:"weight" bson:"weight"`
	CreatedAt        time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at" bson:"updated_at"`
}

func (s *service) registerUpstream(source *upstream) {
	s.Lock()
	defer s.Unlock()

	source.UpdatedAt = time.Now().UTC()
	for _, u := range s.Upstreams {
		if u.Name == source.Name {
			u.TargetURL = source.TargetURL
			return
		}
	}
	s.Upstreams = append(s.Upstreams, source)
}

func (s *service) unregisterUpstream(source *upstream) {
	s.Lock()
	defer s.Unlock()

	for i, u := range s.Upstreams {
		if u.Name == source.Name {
			// remove upstream
			s.Upstreams = append(s.Upstreams[:i], s.Upstreams[i+1:]...)
			return
		}
	}
}

func (s *service) askForUpstream() *upstream {
	s.Lock()
	defer s.Unlock()

	if len(s.Upstreams) == 1 {
		return s.Upstreams[0]
	}
	var result *upstream
	for _, u := range s.Upstreams {
		if u.count == 0 {
			u.count++
			result = u
			break
		}
	}
	// reset count
	if result == nil {
		for _, u := range s.Upstreams {
			u.count = 0
		}
		for _, u := range s.Upstreams {
			if u.count == 0 {
				u.count++
				result = u
				break
			}
		}
	}
	return result
}

func (*service) isValid() bool {
	return true
}

func (a service) isAllow(consumer Consumer) bool {
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
}

type ServiceRepository interface {
	Get(id string) (*service, error)
	GetByName(name string) (*service, error)
	GetAll() ([]*service, error)
	Insert(api *service) error
	Update(api *service) error
	Delete(id string) error
}

type serviceMongo struct {
	connectionString string
}

func newAPIMongo(connectionString string) (*serviceMongo, error) {
	session, err := mgo.Dial(connectionString)
	if err != nil {
		panic(err)
	}
	defer session.Close()
	c := session.DB("bifrost").C("services")

	// create index
	nameIdx := mgo.Index{
		Name:       "service_name_idx",
		Key:        []string{"name"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(nameIdx)
	if err != nil {
		return nil, err
	}

	weightIdx := mgo.Index{
		Name:       "service_weight_idx",
		Key:        []string{"-weight", "created_at"},
		Background: true,
		Sparse:     true,
	}
	err = c.EnsureIndex(weightIdx)
	if err != nil {
		return nil, err
	}

	return &serviceMongo{
		connectionString: connectionString,
	}, nil
}

func (ams *serviceMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(ams.connectionString)
}

func (ams *serviceMongo) Get(id string) (*service, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	api := service{}
	err = c.FindId(id).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *serviceMongo) GetByName(name string) (*service, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	api := service{}
	err = c.Find(bson.M{"name": name}).One(&api)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &api, nil
}

func (ams *serviceMongo) GetAll() ([]*service, error) {
	session, err := ams.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	apis := []*service{}
	err = c.Find(bson.M{}).Sort("-weight", "+created_at").All(&apis)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return apis, nil
}

func (ams *serviceMongo) Insert(api *service) error {
	session, err := ams.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
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

func (ams *serviceMongo) Update(api *service) error {
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

	c := session.DB("bifrost").C("services")
	colQuerier := bson.M{"_id": api.ID}
	err = c.Update(colQuerier, api)
	if err != nil {
		return err
	}
	return nil
}

func (ams *serviceMongo) Delete(id string) error {
	session, err := ams.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	colQuerier := bson.M{"_id": id}
	err = c.Remove(colQuerier)
	if err != nil {
		return err
	}
	return nil
}
