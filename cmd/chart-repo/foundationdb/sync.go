/*
Copyright (c) 2018 The Helm Authors

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

package foundationdb

import (
	"context"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

//SyncCmd Add a new chart repository to FoundationDB and periodically sync it
var SyncCmd = &cobra.Command{
	Use:   "sync [REPO NAME] [REPO URL]",
	Short: "add a new chart repository, and resync its charts periodically",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			log.Info("Need exactly two arguments: [REPO NAME] [REPO URL]")
			cmd.Help()
			return
		}

		fdbURL, err := cmd.Flags().GetString("foundation-url")
		if err != nil {
			log.Fatal(err)
		}
		fDB, err := cmd.Flags().GetString("doclayer-database")
		if err != nil {
			log.Fatal(err)
		}
		// fdbUser, err := cmd.Flags().GetString("fdb-user")
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// fdbPW := os.Getenv("FDB_PASSWORD")
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			log.Fatal(err)
		}
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(fdbURL))
		ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
		err = client.Ping(ctx, readpref.Primary())
		if err != nil {
			log.Fatalf("Can't connect to FoundationDB document layer: %v", err)
		} else {
			log.Info("Successfully connected to FoundationDB document layer.")
		}

		log.Debug("Runnning a quick insert test...")
		collection := client.Database("testing").Collection("numbers")
		ctx, _ = context.WithTimeout(context.Background(), 5*time.Second)
		res, err := collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
		if err != nil {
			log.Fatalf("Failed to insert document")
		}
		id := res.InsertedID
		log.Debugf("Insert test successful: inserted doc with ID: %v", id)

		authorizationHeader := os.Getenv("AUTHORIZATION_HEADER")
		if err = syncRepo(client, fDB, args[0], args[1], authorizationHeader); err != nil {
			log.Fatalf("Can't add chart repository to database: %v", err)
		}

		log.Infof("Successfully added the chart repository %s to database", args[0])
	},
}
