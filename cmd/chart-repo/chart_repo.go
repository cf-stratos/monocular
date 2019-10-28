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

package main

import (
	"os"

	"local/monocular/cmd/chart-repo/foundationdb"
	"local/monocular/cmd/chart-repo/utils"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chart-repo",
	Short: "Chart Repository utility",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func main() {
	cmd := rootCmd
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cmds := []*cobra.Command{foundationdb.SyncCmd, foundationdb.DeleteCmd}

	for _, cmd := range cmds {
		rootCmd.AddCommand(cmd)
		cmd.Flags().String("db-type", "foundation-db", "Database backend. One of either: \"foundation-db\" or \"mongo-db\"")
		cmd.Flags().String("mongo-url", "mongodb://fdb-service/27016", "FoundationDB URL (see https://godoc.org/github.com/globalsign/mgo#Dial for format)")
		cmd.Flags().String("mongo-database", "charts", "FoundationDB Document database")
		cmd.Flags().String("mongo-user", "", "FoundationDB user")

		//cmd.Flags().String("mongo-url", "localhost", "MongoDB URL (see https://godoc.org/github.com/globalsign/mgo#Dial for format)")
		//cmd.Flags().String("mongo-database", "charts", "MongoDB database")
		//cmd.Flags().String("mongo-user", "", "MongoDB user")
		// see version.go
		cmd.Flags().StringVarP(&utils.UserAgentComment, "user-agent-comment", "", "", "UserAgent comment used during outbound requests")
		cmd.Flags().Bool("debug", false, "verbose logging")
	}
	rootCmd.AddCommand(utils.VersionCmd)
}
