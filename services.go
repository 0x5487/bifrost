package main

import (
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	redis "gopkg.in/redis.v4"
)

type upstream struct {
	count         int       `json:"-" bson:"-"`
	Name          string    `json:"name" bson:"-"`
	TargetURL     string    `json:"target_url" bson:"-"`
	TotalRequests uint64    `json:"total_requests" bson:"-"`
	UpdatedAt     time.Time `json:"updated_at" bson:"-"`
	State         string    `json:"state" bson:"-"`
}

type service struct {
	sync.RWMutex `json:"-" bson:"-"`
	ID           string      `json:"id" bson:"_id"`
	Name         string      `json:"name" `
	Port         int         `json:"port" `
	Upstreams    []*upstream `json:"upstreams"`
	CreatedAt    time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" bson:"updated_at"`
}

func newServiceCollection() *serviceCollection {
	return &serviceCollection{
		Count:    0,
		Services: []*service{},
	}
}

func (s *service) registerUpstream(source *upstream) {
	s.Lock()
	defer s.Unlock()

	// update upstream
	for _, u := range s.Upstreams {
		if u.Name == source.Name {
			u.UpdatedAt = time.Now().UTC()
			u.TargetURL = source.TargetURL
			return
		}
	}

	// add upstream
	source.UpdatedAt = time.Now().UTC()
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

	var result *upstream
	if len(s.Upstreams) == 1 {
		result = s.Upstreams[0]
		result.TotalRequests++
		return result
	}

	for _, u := range s.Upstreams {
		if u.count == 0 {
			u.TotalRequests++
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
				u.TotalRequests++
				u.count++
				result = u
				break
			}
		}
	}
	return result
}

type serviceCollection struct {
	Count    int        `json:"count"`
	Services []*service `json:"services"`
}

type ServiceRepository interface {
	Get(id string) (*service, error)
	GetByName(name string) (*service, error)
	GetAll() ([]*service, error)
	Insert(source *service) error
	Update(source *service) error
	Delete(id string) error
}

/*********************
	Mongo Database
*********************/

type serviceMongo struct {
	connectionString string
}

func newServiceMongo(connectionString string) (*serviceMongo, error) {
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

	return &serviceMongo{
		connectionString: connectionString,
	}, nil
}

func (sm *serviceMongo) newSession() (*mgo.Session, error) {
	return mgo.Dial(sm.connectionString)
}

func (sm *serviceMongo) Get(id string) (*service, error) {
	session, err := sm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	service := service{}
	err = c.FindId(id).One(&service)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &service, nil
}

func (sm *serviceMongo) GetByName(name string) (*service, error) {
	session, err := sm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	service := service{}
	err = c.Find(bson.M{"name": name}).One(&service)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return &service, nil
}

func (sm *serviceMongo) GetAll() ([]*service, error) {
	session, err := sm.newSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	services := []*service{}
	err = c.Find(bson.M{}).All(&services)
	if err != nil {
		if err.Error() == "not found" {
			return nil, nil
		}
		return nil, err
	}
	return services, nil
}

func (sm *serviceMongo) Insert(source *service) error {
	session, err := sm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	source.ID = uuid.NewV4().String()
	now := time.Now().UTC()
	source.CreatedAt = now
	source.UpdatedAt = now
	err = c.Insert(source)

	if err != nil {
		if strings.HasPrefix(err.Error(), "E11000") {
			return AppError{ErrorCode: "invalid_data", Message: "The service already exits"}
		}
		return err
	}
	return nil
}

func (sm *serviceMongo) Update(source *service) error {
	if len(source.ID) == 0 {
		return AppError{ErrorCode: "invalid_data", Message: "id can't be empty or null."}
	}
	now := time.Now().UTC()
	source.UpdatedAt = now

	session, err := sm.newSession()
	if err != nil {
		return err
	}
	defer session.Close()

	c := session.DB("bifrost").C("services")
	colQuerier := bson.M{"_id": source.ID}
	err = c.Update(colQuerier, source)
	if err != nil {
		return err
	}
	return nil
}

func (sm *serviceMongo) Delete(id string) error {
	session, err := sm.newSession()
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

/*********************
	Redis Database
*********************/

type serviceRedis struct {
	client *redis.Client
}

func newServiceRedis(addr string, password string, db int) (*serviceRedis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	serviceRedis := &serviceRedis{
		client: client,
	}
	return serviceRedis, nil
}

func (source *serviceRedis) Get(id string) (*service, error) {
	/*
		session, err := sm.newSession()
		if err != nil {
			return nil, err
		}
		defer session.Close()

		c := session.DB("bifrost").C("services")
		service := service{}
		err = c.FindId(id).One(&service)
		if err != nil {
			if err.Error() == "not found" {
				return nil, nil
			}
			return nil, err
		}
	*/
	return nil, nil
}

func (source *serviceRedis) GetByName(name string) (*service, error) {
	/*
		session, err := sm.newSession()
		if err != nil {
			return nil, err
		}
		defer session.Close()

		c := session.DB("bifrost").C("services")
		service := service{}
		err = c.Find(bson.M{"name": name}).One(&service)
		if err != nil {
			if err.Error() == "not found" {
				return nil, nil
			}
			return nil, err
		}
	*/
	return nil, nil
}

func (source *serviceRedis) GetAll() ([]*service, error) {
	/*
		session, err := sm.newSession()
		if err != nil {
			return nil, err
		}
		defer session.Close()

		c := session.DB("bifrost").C("services")
		services := []*service{}
		err = c.Find(bson.M{}).All(&services)
		if err != nil {
			if err.Error() == "not found" {
				return nil, nil
			}
			return nil, err
		}
	*/
	return nil, nil
}

func (srouce *serviceRedis) Insert(svc *service) error {
	/*
		session, err := sm.newSession()
		if err != nil {
			return err
		}
		defer session.Close()

		c := session.DB("bifrost").C("services")
		source.ID = uuid.NewV4().String()
		now := time.Now().UTC()
		source.CreatedAt = now
		source.UpdatedAt = now
		err = c.Insert(source)

		if err != nil {
			if strings.HasPrefix(err.Error(), "E11000") {
				return AppError{ErrorCode: "invalid_data", Message: "The service already exits"}
			}
			return err
		}
	*/
	return nil
}

func (source *serviceRedis) Update(svc *service) error {
	/*
		if len(source.ID) == 0 {
			return AppError{ErrorCode: "invalid_data", Message: "id can't be empty or null."}
		}
		now := time.Now().UTC()
		source.UpdatedAt = now

		session, err := sm.newSession()
		if err != nil {
			return err
		}
		defer session.Close()

		c := session.DB("bifrost").C("services")
		colQuerier := bson.M{"_id": source.ID}
		err = c.Update(colQuerier, source)
		if err != nil {
			return err
		}
	*/
	return nil
}

func (source *serviceRedis) Delete(id string) error {
	/*
		session, err := sm.newSession()
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
	*/
	return nil
}
