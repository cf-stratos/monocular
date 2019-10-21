package foundationdb

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func Database(client *mongo.Client, dbName string) (*mongo.Database, func()) {

	db := client.Database(dbName)
	return db, func() {
		err := client.Disconnect(context.TODO())

		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Connection to MongoDB closed.")
	}
}
