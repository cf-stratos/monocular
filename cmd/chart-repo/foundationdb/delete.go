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
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

//DeleteCmd Delete a chart repository from FoundationDB
var DeleteCmd = &cobra.Command{
	Use:   "delete [REPO NAME]",
	Short: "delete a chart repository",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			log.Info("Need exactly one argument: [REPO NAME]")
			cmd.Help()
			return
		}
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

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(fdbURL))
		ctx, _ = context.WithTimeout(context.Background(), 2*time.Second)
		err = client.Ping(ctx, readpref.Primary())
		if err != nil {
			log.Fatalf("Can't connect to FoundationDB document layer: %v", err)
		} else {
			log.Info("Successfully connected to FoundationDB document layer.")
		}

		if err = deleteRepo(client, fDB, args[0]); err != nil {
			log.Fatalf("Can't delete chart repository %s from database: %v", args[0], err)
		}

		log.Infof("Successfully deleted the chart repository %s from database", args[0])
	},
}
