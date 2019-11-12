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
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	Collection(name string) Collection
}

type mongoBulkWriteResult struct {
	BulkWriteResult mongo.BulkWriteResult
}

// Collection is an interface for accessing a MongoDB collection
type Collection interface {
	BulkWrite(ctxt context.Context, operations mongoWriteModels, options mongoBulkWriteOptions) (MongoResult, error)
	DeleteMany(ctxt context.Context, filter interface{}, options mongoDeleteOptions) (MongoResult, error)
	FindOne(ctxt context.Context, filter interface{}, options mongoFindOneOptions) MongoResult
	InsertOne(ctxt context.Context, document interface{}, options mongoInsertOneOptions) (MongoResult, error)
}

type MongoResult interface{}

func DB(client *mongo.Client, dbName string) (Database, func()) {

	db := &mongoDatabase{client.Database(dbName)}

	return db, func() {
		err := client.Disconnect(context.Background())

		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Connection to MongoDB closed.")
	}
}

// mgoDatabase wraps an mgo.Database and implements Database
type mongoDatabase struct {
	Database *mongo.Database
}

func (d *mongoDatabase) Collection(name string) Collection {
	return &mongoCollection{d.Database.Collection(name)}
}

// mgoCollection wraps an mgo.Collection and implements Collection
type mongoCollection struct {
	Collection *mongo.Collection
}

type mongoFindOneOptions struct {
	FindOneOptions *options.FindOneOptions
}

type mongoDeleteOptions struct {
	DeleteOptions *options.DeleteOptions
}

type mongoBulkWriteOptions struct {
	BulkWriteOptions *options.BulkWriteOptions
}

type mongoInsertOneOptions struct {
	InsertOneOptions *options.InsertOneOptions
}

type mongoWriteModels struct {
	WriteModels []mongo.WriteModel
}

func (c *mongoCollection) BulkWrite(ctxt context.Context, operations mongoWriteModels, options mongoBulkWriteOptions) (MongoResult, error) {
	return c.Collection.BulkWrite(ctxt, operations.WriteModels, options.BulkWriteOptions)
}

func (c *mongoCollection) DeleteMany(ctxt context.Context, filter interface{}, options mongoDeleteOptions) (MongoResult, error) {
	return c.Collection.DeleteMany(ctxt, filter, options.DeleteOptions)
}

func (c *mongoCollection) FindOne(ctxt context.Context, filter interface{}, options mongoFindOneOptions) MongoResult {
	return c.Collection.FindOne(ctxt, filter, options.FindOneOptions)
}

func (c *mongoCollection) InsertOne(ctxt context.Context, document interface{}, options mongoInsertOneOptions) (MongoResult, error) {
	return c.Collection.InsertOne(ctxt, document, options.InsertOneOptions)
}
