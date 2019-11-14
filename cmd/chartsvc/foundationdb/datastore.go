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
	"reflect"
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
	Database(name string) (Database, func())
}

// Database is an interface for accessing a MongoDB database
type Database interface {
	Collection(name string) Collection
}

// Collection is an interface for accessing a MongoDB collection
type Collection interface {
	BulkWrite(ctxt context.Context, operations []mongo.WriteModel, options *options.BulkWriteOptions) (*mongo.BulkWriteResult, error)
	DeleteMany(ctxt context.Context, filter interface{}, options *options.DeleteOptions) (*mongo.DeleteResult, error)
	FindOne(ctxt context.Context, filter interface{}, result interface{}, options *options.FindOneOptions) error
	InsertOne(ctxt context.Context, document interface{}, options *options.InsertOneOptions) (*mongo.InsertOneResult, error)
	UpdateOne(ctxt context.Context, filter interface{}, update interface{}, options *options.UpdateOptions) (*mongo.UpdateResult, error)
	Find(ctxt context.Context, filter interface{}, options *options.FindOptions) (*mongo.Cursor, error)
}

// mgoDatabase wraps an mgo.Database and implements Database
type mongoDatabase struct {
	Database *mongo.Database
}

// mgoDatabase wraps an mgo.Database and implements Database
type mongoClient struct {
	Client *mongo.Client
}

func NewDocLayerClient(ctx context.Context, options *options.ClientOptions) (Client, error) {
	client, err := mongo.Connect(ctx, options)
	return &mongoClient{client}, err
}

func (c *mongoClient) Database(dbName string) (Database, func()) {

	db := &mongoDatabase{c.Client.Database(dbName)}

	return db, func() {
		err := c.Client.Disconnect(context.Background())

		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Connection to MongoDB closed.")
	}
}

func (d *mongoDatabase) Collection(name string) Collection {
	return &mongoCollection{d.Database.Collection(name)}
}

// mgoCollection wraps an mgo.Collection and implements Collection
type mongoCollection struct {
	Collection *mongo.Collection
}

// type mongoFindOneOptions struct {
// 	FindOneOptions *options.FindOneOptions
// }

// type mongoDeleteOptions struct {
// 	DeleteOptions *options.DeleteOptions
// }

// type mongoBulkWriteOptions struct {
// 	BulkWriteOptions *options.BulkWriteOptions
// }

// type mongoInsertOneOptions struct {
// 	InsertOneOptions *options.InsertOneOptions
// }

// type mongoUpdateOptions struct {
// 	UpdateOneOptions *options.UpdateOptions
// }

// type mongoFindOneResult struct {
// 	FindOneResult *mongo.SingleResult
// }

// type mongoDeleteResult struct {
// 	DeleteResult *mongo.DeleteResult
// }

// type mongoBulkWriteResult struct {
// 	BulkWriteResult *mongo.BulkWriteResult
// }

// type mongoInsertOneResult struct {
// 	InsertOneResult *mongo.InsertOneResult
// }

// type mongoUpdateResult struct {
// 	UpdateOneResult *mongo.UpdateResult
// }

// type mongoWriteModels struct {
// 	WriteModels *[]mongo.WriteModel
// }

func (c *mongoCollection) BulkWrite(ctxt context.Context, operations []mongo.WriteModel, options *options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	res, err := c.Collection.BulkWrite(ctxt, operations, options)
	return res, err
}

func (c *mongoCollection) DeleteMany(ctxt context.Context, filter interface{}, options *options.DeleteOptions) (*mongo.DeleteResult, error) {
	res, err := c.Collection.DeleteMany(ctxt, filter, options)
	return res, err
}

func (c *mongoCollection) FindOne(ctxt context.Context, filter interface{}, result interface{}, options *options.FindOneOptions) error {
	res := c.Collection.FindOne(ctxt, filter, options)
	return res.Decode(result)
}

func (c *mongoCollection) InsertOne(ctxt context.Context, document interface{}, options *options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	res, err := c.Collection.InsertOne(ctxt, document, options)
	return res, err
}

func (c *mongoCollection) UpdateOne(ctxt context.Context, filter interface{}, document interface{}, options *options.UpdateOptions) (*mongo.UpdateResult, error) {
	res, err := c.Collection.UpdateOne(ctxt, filter, document, options)
	return res, err
}

func (c *mongoCollection) Find(ctxt context.Context, filter interface{}, result *[]interface{}, options *options.FindOptions) (interface{}, error) {
	resCursor, err := c.Collection.Find(ctxt, filter, options)
	resultType := reflect.ValueOf(result)
	if resultType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Result arg must be a pointer type. Got: %v", resultType.Kind())
	}
	resultSliceType := resultType.Elem()
	if resultSliceType.Kind() != reflect.Slice {
		return nil, fmt.Errorf("Result arg must be a slice value. Got: %v", resultSliceType.Kind())
	}
	resultElementType := resultSliceType.Type()
	resultSlice := reflect.MakeSlice(resultElementType, 1, 1)

	for resCursor.Next(context.Background()) {
		newElement := reflect.New(resultElementType)
		// Decode the document
		if err := resCursor.Decode(&newElement); err != nil {
			log.WithError(err).Errorf(
				"Error decoding element in find results.",
			)
			continue
		}
		reflect.AppendSlice(resultSlice, newElement)
	}
	reflect.ValueOf(result).Elem().Set(resultSlice)
	return resultSlice.Interface, err
}
