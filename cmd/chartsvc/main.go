/*
Copyright (c) 2017 The Helm Authors

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
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/heptiolabs/healthcheck"
	"github.com/kubeapps/common/datastore"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/negroni"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	fdb "local/monocular/cmd/chartsvc/foundationdb"
)

const pathPrefix = "/v1"

var client *mongo.Client
var dbSession datastore.Session

func setupRoutes() http.Handler {
	r := mux.NewRouter()

	// Healthcheck
	health := healthcheck.NewHandler()
	r.Handle("/live", health)
	r.Handle("/ready", health)

	// Routes
	apiv1 := r.PathPrefix(pathPrefix).Subrouter()
	apiv1.Methods("GET").Path("/charts").Queries("name", "{chartName}", "version", "{version}", "appversion", "{appversion}").Handler(fdb.WithParams(fdb.ListChartsWithFilters))
	apiv1.Methods("GET").Path("/charts").HandlerFunc(fdb.ListCharts)
	apiv1.Methods("GET").Path("/charts/search").Queries("q", "{query}").Handler(fdb.WithParams(fdb.SearchCharts))
	apiv1.Methods("GET").Path("/charts/{repo}").Handler(fdb.WithParams(fdb.ListRepoCharts))
	apiv1.Methods("GET").Path("/charts/{repo}/search").Queries("q", "{query}").Handler(fdb.WithParams(fdb.SearchCharts))
	apiv1.Methods("GET").Path("/charts/{repo}/{chartName}").Handler(fdb.WithParams(fdb.GetChart))
	apiv1.Methods("GET").Path("/charts/{repo}/{chartName}/versions").Handler(fdb.WithParams(fdb.ListChartVersions))
	apiv1.Methods("GET").Path("/charts/{repo}/{chartName}/versions/{version}").Handler(fdb.WithParams(fdb.GetChartVersion))
	apiv1.Methods("GET").Path("/assets/{repo}/{chartName}/logo-160x160-fit.png").Handler(fdb.WithParams(fdb.GetChartIcon))
	apiv1.Methods("GET").Path("/assets/{repo}/{chartName}/versions/{version}/README.md").Handler(fdb.WithParams(fdb.GetChartVersionReadme))
	apiv1.Methods("GET").Path("/assets/{repo}/{chartName}/versions/{version}/values.yaml").Handler(fdb.WithParams(fdb.GetChartVersionValues))

	n := negroni.Classic()
	n.UseHandler(r)
	return n
}

func main() {

	fdbURL := flag.String("mongo-url", "mongodb://fdb-service/27016", "MongoDB URL (see https://godoc.org/github.com/globalsign/mgo#Dial for format)")
	fDB := flag.String("mongo-database", "charts", "MongoDB database")
	debug := flag.Bool("debug", true, "Debug Logging")
	flag.Parse()
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	log.Infof("Attempting to connect to FDB: %v, %v, %v", *fdbURL, *fDB, *debug)

	clientOptions := options.Client().ApplyURI(*fdbURL)
	client, err := fdb.NewDocLayerClient(context.Background(), clientOptions)
	if err != nil {
		log.Fatalf("Can't create client for FoundationDB document layer: %v", err)
		return
	} else {
		log.Infof("Client created.")
	}

	fdb.InitDBConfig(client, *fDB)
	fdb.SetPathPrefix(pathPrefix)

	n := setupRoutes()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.WithFields(log.Fields{"addr": addr}).Info("Started chartsvc")
	http.ListenAndServe(addr, n)
}
