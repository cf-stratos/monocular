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

package cmd

import (
	"context"
	"local/monocular/cmd/chart-repo/foundationdb"
	"local/monocular/cmd/chart-repo/mongodb"
	"os"
	"time"

	"github.com/kubeapps/common/datastore"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var SyncCmd = &cobra.Command{
	Use:   "sync [REPO NAME] [REPO URL]",
	Short: "add a new chart repository, and resync its charts periodically",
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) != 2 {
			log.Info("Need exactly two arguments: [REPO NAME] [REPO URL]")
			cmd.Help()
			return
		}

		dbType, err := cmd.Flags().GetString("db-type")
		if err != nil {
			runMongoDBSync(cmd, args)
		}

		switch dbType {
		case "mongodb":
			runMongoDBSync(cmd, args)
		case "fdb":
			runFDBSync(cmd, args)
		default:
			log.Fatalf("Unknown database type: %v. db-type, if set, must be either 'mongodb' or 'fdb'.", dbType)
		}
	},
}

func runMongoDBSync(cmd *cobra.Command, args []string) {

	mongoURL, err := cmd.Flags().GetString("mongo-url")
	if err != nil {
		log.Fatal(err)
	}
	mongoDB, err := cmd.Flags().GetString("mongo-database")
	if err != nil {
		log.Fatal(err)
	}
	mongoUser, err := cmd.Flags().GetString("mongo-user")
	if err != nil {
		log.Fatal(err)
	}
	mongoPW := os.Getenv("MONGO_PASSWORD")
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		log.SetLevel(logrus.DebugLevel)
	}
	mongoConfig := datastore.Config{URL: mongoURL, Database: mongoDB, Username: mongoUser, Password: mongoPW}
	dbSession, err := datastore.NewSession(mongoConfig)
	if err != nil {
		log.Fatalf("Can't connect to mongoDB: %v", err)
	}

	authorizationHeader := os.Getenv("AUTHORIZATION_HEADER")
	if err = mongodb.SyncRepo(dbSession, args[0], args[1], authorizationHeader); err != nil {
		logrus.Fatalf("Can't add chart repository to database: %v", err)
	}

	logrus.Infof("Successfully added the chart repository %s to database", args[0])
}

func runFDBSync(cmd *cobra.Command, args []string) {

	fdbURL, err := cmd.Flags().GetString("doclayer-url")
	if err != nil {
		log.Fatal(err)
	}
	fDB, err := cmd.Flags().GetString("doclayer-database")
	if err != nil {
		log.Fatal(err)
	}
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Infof("Creating client for FDB: %v, %v, %v", fdbURL, fDB, debug)
	clientOptions := options.Client().ApplyURI(fdbURL).SetMinPoolSize(10).SetMaxPoolSize(100)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Can't create client for FoundationDB document layer: %v", err)
		return
	} else {
		log.Infof("Client created.")
	}
	startTime := time.Now()
	authorizationHeader := os.Getenv("AUTHORIZATION_HEADER")
	if err = foundationdb.SyncRepo(client, fDB, args[0], args[1], authorizationHeader); err != nil {
		log.Fatalf("Can't add chart repository to database: %v", err)
		return
	}
	timeTaken := time.Since(startTime).Seconds()
	log.Infof("Successfully added the chart repository %s to database in %v seconds", args[0], timeTaken)
}
