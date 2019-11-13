package foundationdb

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
)

// mockDatabase acts as a mock datastore.Database
type mockDatabase struct {
	*mock.Mock
}

type mockClient struct {
	*mock.Mock
}

// NewMockSession returns a mocked Session
func NewMockClient(m *mock.Mock) Client {
	return mockClient{m}
}

// DB returns a mocked datastore.Database and empty closer function
func (c mockClient) Database(dbName string) (Database, func()) {

	db := &mockDatabase{c.Mock}

	return db, func() {
	}
}

func (d mockDatabase) Collection(name string) Collection {
	return &mockCollection{d.Mock}
}

// mockCollection acts as a mock datastore.Collection
type mockCollection struct {
	*mock.Mock
}

/* // mockBulkWriteReult acts as a mock datastore.Collection
type mockBulkWriteResult struct {
	*mock.Mock
}

// mockDeleteManyResult acts as a mock datastore.Collection
type mockDeleteResult struct {
	*mock.Mock
}

// mockFindOneResult acts as a mock datastore.Collection
type mockFindOneResult struct {
	*mock.Mock
}

// mockCollection acts as a mock datastore.Collection
type mockInsertOneResult struct {
	*mock.Mock
} */

func (c mockCollection) BulkWrite(ctxt context.Context, operations []mongo.WriteModel, options *mongoBulkWriteOptions) (*mongo.BulkWriteResult, error) {
	args := c.Called(ctxt, operations, options)
	return args.Get(0).(*mongo.BulkWriteResult), args.Error(1)
}

func (c mockCollection) DeleteMany(ctxt context.Context, filter interface{}, options *mongoDeleteOptions) (*mongo.DeleteResult, error) {
	args := c.Called(ctxt, filter, options)
	return args.Get(0).(*mongo.DeleteResult), args.Error(1)
}

func (c mockCollection) FindOne(ctxt context.Context, filter interface{}, options *mongoFindOneOptions) *mongo.SingleResult {
	args := c.Called(ctxt, filter, options)
	return args.Get(0).(*mongo.SingleResult)
}

func (c mockCollection) InsertOne(ctxt context.Context, document interface{}, options *mongoInsertOneOptions) (*mongo.InsertOneResult, error) {
	args := c.Called(ctxt, document, options)
	return args.Get(0).(*mongo.InsertOneResult), args.Error(1)
}

func (c mockCollection) UpdateOne(ctxt context.Context, filter interface{}, document interface{}, options *mongoUpdateOptions) (*mongo.UpdateResult, error) {
	args := c.Called(ctxt, document, options)
	return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}
