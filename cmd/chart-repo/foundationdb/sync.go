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
	"os"

	"github.com/kubeapps/common/datastore"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//SyncCmd Add a new chart repository to FoundationDB and periodically sync it
var SyncCmd = &cobra.Command{
	Use:   "sync [REPO NAME] [REPO URL]",
	Short: "add a new chart repository, and resync its charts periodically",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			logrus.Info("Need exactly two arguments: [REPO NAME] [REPO URL]")
			cmd.Help()
			return
		}

		fdbURL, err := cmd.Flags().GetString("foundation-url")
		if err != nil {
			logrus.Fatal(err)
		}
		fDB, err := cmd.Flags().GetString("doclayer-database")
		if err != nil {
			logrus.Fatal(err)
		}
		fdbUser, err := cmd.Flags().GetString("fdb-user")
		if err != nil {
			logrus.Fatal(err)
		}
		fdbPW := os.Getenv("FDB_PASSWORD")
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			logrus.Fatal(err)
		}
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		mongoConfig := datastore.Config{URL: fdbURL, Database: fDB, Username: fdbUser, Password: fdbPW}
		dbSession, err := datastore.NewSession(mongoConfig)
		if err != nil {
			logrus.Fatalf("Can't connect to FoundationDB: %v", err)
		}

		authorizationHeader := os.Getenv("AUTHORIZATION_HEADER")
		if err = syncRepo(dbSession, args[0], args[1], authorizationHeader); err != nil {
			logrus.Fatalf("Can't add chart repository to database: %v", err)
		}

		logrus.Infof("Successfully added the chart repository %s to database", args[0])
	},
}
