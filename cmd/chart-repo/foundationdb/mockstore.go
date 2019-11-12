package foundationdb

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// mockDatabase acts as a mock datastore.Database
type mockDatabase struct {
	*mock.Mock
}

func (d mockDatabase) Collection(name string) Collection {
	return mockCollection{d.Mock}
}

// mockCollection acts as a mock datastore.Collection
type mockCollection struct {
	*mock.Mock
}

// mockBulkWriteReult acts as a mock datastore.Collection
type mockBulkWriteResult struct {
	*mock.Mock
}

// mockDeleteManyResult acts as a mock datastore.Collection
type mockDeleteManyResult struct {
	*mock.Mock
}

// mockFindOneResult acts as a mock datastore.Collection
type mockFindOneResult struct {
	*mock.Mock
}

// mockCollection acts as a mock datastore.Collection
type mockInsertOneResult struct {
	*mock.Mock
}

func (c mockCollection) BulkWrite(ctxt context.Context, operations mongoWriteModels, options mongoBulkWriteOptions) (MongoResult, error) {
	c.Called(ctxt, operations, options)
	return mockBulkWriteResult{c.Mock}, nil
}

func (c mockCollection) DeleteMany(ctxt context.Context, filter interface{}, options mongoDeleteOptions) (MongoResult, error) {
	c.Called(ctxt, filter, options)
	return mockDeleteManyResult{c.Mock}, nil
}

func (c mockCollection) FindOne(ctxt context.Context, filter interface{}, options mongoFindOneOptions) MongoResult {
	c.Called(ctxt, filter, options)
	return mockFindOneResult{c.Mock}
}

func (c mockCollection) InsertOne(ctxt context.Context, document interface{}, options mongoInsertOneOptions) (MongoResult, error) {
	c.Called(ctxt, document, options)
	return mockFindOneResult{c.Mock}, nil
}

// NewMockSession returns a mocked Session
func NewMockDB(m *mock.Mock) Database {
	return mockDatabase{m}
}
