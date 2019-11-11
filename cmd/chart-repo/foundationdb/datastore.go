/*
Copyright (c) 2016-2017 Bitnami

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package datastore implements an interface on top of the mgo mongo client
package foundationdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/globalsign/mgo"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

const defaultTimeout = 30 * time.Second

// Config configures the database connection
type Config struct {
	URL      string
	Database string
	Timeout  time.Duration
}

// Client is an interface for a MongoDB client
type Client interface {
	DB() (Database, func())
	Use(name string) Client
}

// Database is an interface for accessing a MongoDB database
type Database interface {
	C(name string) Collection
}

// Collection is an interface for accessing a MongoDB collection
type Collection interface {
	Bulk() Bulk
	Pipe(pipeline interface{}) Pipe
	Find(query interface{}) Query
	FindId(id interface{}) Query
	Count() (n int, err error)
	Insert(docs ...interface{}) error
	Remove(selector interface{}) error
	RemoveAll(selector interface{}) (*mgo.ChangeInfo, error)
	UpdateId(id, update interface{}) error
	Upsert(selector, update interface{}) (*mgo.ChangeInfo, error)
	UpsertId(id, update interface{}) (*mgo.ChangeInfo, error)
}

// Bulk is an interface for running Bulk queries on a MongoDB collection
type Bulk interface {
	Upsert(pairs ...interface{})
	RemoveAll(selectors ...interface{})
	Run() (*mgo.BulkResult, error)
}

// Query is an interface for querying a MongoDB collection
type Query interface {
	All(result interface{}) error
	One(result interface{}) error
	Sort(fields ...string) Query
	Select(selector interface{}) Query
}

// Pipe is an interface for MongoDB aggregation
type Pipe interface {
	All(result interface{}) error
	One(result interface{}) error
}

// mgoSession wraps an mgo.Session and implements Session
type mgoSession struct {
	conf Config
	*mgo.Session
}

func DB(client *mongo.Client, dbName string) (Database, func()) {

	db := &mongoDatabase{client.Database(dbName)}

	return db, func() {
		err := client.Disconnect(context.Background())

		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Connection to MongoDB closed.")
	}
}s

// mgoDatabase wraps an mgo.Database and implements Database
type mongoDatabase struct {
	*mongo.Database
}

func (d *mongoDatabase) C(name string) Collection {
	return &mgoCollection{d.Database.C(name)}
}

// mgoCollection wraps an mgo.Collection and implements Collection
type mongoCollection struct {
	*mongo.Collection
}

func (c *mongoCollection) Bulk() Bulk {
	return c.Collection.Bulk()
}

func (c *mongoCollection) Find(query interface{}) Query {
	return &mongoQuery{c.Collection.Find(query)}
}

func (c *mongoCollection) FindId(id interface{}) Query {
	return &mgoQuery{c.Collection.FindId(id)}
}

// mgoQuery wraps an mgo.Query and implements Query
type mgoQuery struct {
	*mongo.Query
}

func (q *mgoQuery) Sort(fields ...string) Query {
	return &mgoQuery{q.Query.Sort(fields...)}
}

func (q *mgoQuery) Select(selector interface{}) Query {
	return &mgoQuery{q.Query.Select(selector)}
}

// NewSession initializes a MongoDB connection to the given host
func NewSession(conf Config) (Session, error) {
	dialInfo, err := mgo.ParseURL(conf.URL)
	if err != nil {
		return nil, err
	}
	if conf.Username != "" {
		dialInfo.Username = conf.Username
	}
	if conf.Password != "" {
		dialInfo.Password = conf.Password
	}
	if conf.Timeout == 0 {
		conf.Timeout = defaultTimeout
	}
	dialInfo.Timeout = conf.Timeout
	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		return nil, errors.New("unable to connect to MongoDB")
	}
	session.SetMode(mgo.Monotonic, true)
	return &mgoSession{conf, session}, nil
}
