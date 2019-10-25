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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"local/monocular/cmd/chart-repo/types"
	"local/monocular/cmd/chart-repo/utils"

	"github.com/disintegration/imaging"
	"github.com/ghodss/yaml"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	helmrepo "k8s.io/helm/pkg/repo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	chartCollection       = "charts"
	chartFilesCollection  = "files"
	defaultTimeoutSeconds = 10
	additionalCAFile      = "/usr/local/share/ca-certificates/ca.crt"
	dbName                = "test"
)

type importChartFilesJob struct {
	Name         string
	Repo         types.Repo
	ChartVersion types.ChartVersion
}

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var netClient httpClient = &http.Client{}

func parseRepoUrl(repoURL string) (*url.URL, error) {
	repoURL = strings.TrimSpace(repoURL)
	return url.ParseRequestURI(repoURL)
}

func init() {
	var err error
	netClient, err = initNetClient(additionalCAFile)
	if err != nil {
		log.Fatal(err)
	}
}

// SyncRepo Syncing is performed in the following steps:
// 1. Update database to match chart metadata from index
// 2. Concurrently process icons for charts (concurrently)
// 3. Concurrently process the README and values.yaml for the latest chart version of each chart
// 4. Concurrently process READMEs and values.yaml for historic chart versions
//
// These steps are processed in this way to ensure relevant chart data is
// imported into the database as fast as possible. E.g. we want all icons for
// charts before fetching readmes for each chart and version pair.
func syncRepo(dbClient *mongo.Client, dbName, repoName, repoURL string, authorizationHeader string) error {

	db, closer := Database(dbClient, dbName)
	defer closer()

	log.Infof("Running a quick insert test...")
	collection := db.Collection("numbers")
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	res, err := collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
	if err != nil {
		log.Fatalf("Failed to insert document: %v", err)
		return err
	}
	id := res.InsertedID
	log.Debugf("Insert test successful: inserted doc with ID: %v", id)

	log.Infof("TESTING ONLY!: Clearing out all charts and chart files")
	collection = db.Collection(chartFilesCollection)
	_, err = collection.DeleteMany(context.Background(), bson.M{}, options.Delete())
	if err != nil {
		log.Errorf("Error occurred clearing out chart files Err: %v", err)
		return err
	}
	collection = db.Collection(chartCollection)
	_, err = collection.DeleteMany(context.Background(), bson.M{}, options.Delete())
	if err != nil {
		log.Errorf("Error occurred clearing out charts Err: %v", err)
		return err
	}
	log.Infof("TESTING ONLY!: Clearing out all charts and chart files")

	url, err := parseRepoUrl(repoURL)
	if err != nil {
		log.WithFields(log.Fields{"url": repoURL}).WithError(err).Error("failed to parse URL")
		return err
	}

	r := types.Repo{Name: repoName, URL: url.String(), AuthorizationHeader: authorizationHeader}
	index, err := fetchRepoIndex(r)
	if err != nil {
		return err
	}

	charts := chartsFromIndex(index, r)
	if len(charts) == 0 {
		return errors.New("no charts in repository index")
	}
	//TODO Kate REMOVE!
	charts = charts[0:2]
	err = importCharts(db, dbName, charts)
	if err != nil {
		return err
	}

	// Process 10 charts at a time
	numWorkers := 10
	iconJobs := make(chan types.Chart, numWorkers)
	chartFilesJobs := make(chan importChartFilesJob, numWorkers)
	var wg sync.WaitGroup

	log.Debugf("starting %d workers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go importWorker(db, &wg, iconJobs, chartFilesJobs)
	}

	// Enqueue jobs to process chart icons
	for _, c := range charts {
		iconJobs <- c
	}
	// Close the iconJobs channel to signal the worker pools to move on to the
	// chart files jobs
	close(iconJobs)

	// Iterate through the list of charts and enqueue the latest chart version to
	// be processed. Append the rest of the chart versions to a list to be
	// enqueued later
	var toEnqueue []importChartFilesJob
	for _, c := range charts {
		chartFilesJobs <- importChartFilesJob{c.Name, c.Repo, c.ChartVersions[0]}
		for _, cv := range c.ChartVersions[1:] {
			toEnqueue = append(toEnqueue, importChartFilesJob{c.Name, c.Repo, cv})
		}
	}

	// Enqueue all the remaining chart versions
	for _, cfj := range toEnqueue {
		chartFilesJobs <- cfj
	}
	// Close the chartFilesJobs channel to signal the worker pools that there are
	// no more jobs to process
	close(chartFilesJobs)

	// Wait for the worker pools to finish processing
	wg.Wait()

	return nil
}

func deleteRepo(dbClient *mongo.Client, dbName, repoName string) error {
	db, closer := Database(dbClient, dbName)
	defer closer()
	collection := db.Collection(chartCollection)
	filter := bson.M{
		"repo.name": repoName,
	}
	deleteResult, err := collection.DeleteMany(context.Background(), filter, options.Delete())
	if err != nil {
		log.Errorf("Error occurred during delete repo (deleting charts from index). Err: %v, Result: %v", err, deleteResult)
		return err
	}
	log.Debugf("Repo delete (delete charts from index) result: %v charts deleted", deleteResult.DeletedCount)

	collection = db.Collection(chartFilesCollection)
	deleteResult, err = collection.DeleteMany(context.Background(), filter, options.Delete())
	if err != nil {
		log.Errorf("Error occurred during delete repo (deleting chart files from index). Err: %v, Result: %v", err, deleteResult)
		return err
	}
	log.Debugf("Repo delete (delete chart files from index) result: %v chart files deleted.", deleteResult.DeletedCount)
	return err
}

func fetchRepoIndex(r types.Repo) (*helmrepo.IndexFile, error) {
	indexURL, err := parseRepoUrl(r.URL)
	if err != nil {
		log.WithFields(log.Fields{"url": r.URL}).WithError(err).Error("failed to parse URL")
		return nil, err
	}
	indexURL.Path = path.Join(indexURL.Path, "index.yaml")
	req, err := http.NewRequest("GET", indexURL.String(), nil)
	if err != nil {
		log.WithFields(log.Fields{"url": req.URL.String()}).WithError(err).Error("could not build repo index request")
		return nil, err
	}

	req.Header.Set("User-Agent", utils.UserAgent())
	if len(r.AuthorizationHeader) > 0 {
		req.Header.Set("Authorization", r.AuthorizationHeader)
	}

	res, err := netClient.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		log.WithFields(log.Fields{"url": req.URL.String()}).WithError(err).Error("error requesting repo index")
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{"url": req.URL.String(), "status": res.StatusCode}).Error("error requesting repo index, are you sure this is a chart repository?")
		return nil, errors.New("repo index request failed")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return parseRepoIndex(body)
}

func parseRepoIndex(body []byte) (*helmrepo.IndexFile, error) {
	var index helmrepo.IndexFile
	err := yaml.Unmarshal(body, &index)
	if err != nil {
		return nil, err
	}
	index.SortEntries()
	return &index, nil
}

func chartsFromIndex(index *helmrepo.IndexFile, r types.Repo) []types.Chart {
	var charts []types.Chart
	for _, entry := range index.Entries {
		if entry[0].GetDeprecated() {
			log.WithFields(log.Fields{"name": entry[0].GetName()}).Info("skipping deprecated chart")
			continue
		}
		charts = append(charts, newChart(entry, r))
	}
	return charts
}

// Takes an entry from the index and constructs a database representation of the
// object.
func newChart(entry helmrepo.ChartVersions, r types.Repo) types.Chart {
	var c types.Chart
	copier.Copy(&c, entry[0])
	copier.Copy(&c.ChartVersions, entry)
	c.Repo = r
	c.ID = fmt.Sprintf("%s/%s", r.Name, c.Name)
	return c
}

func importCharts(db *mongo.Database, dbName string, charts []types.Chart) error {
	var operations []mongo.WriteModel
	operation := mongo.NewUpdateOneModel()
	var chartIDs []string
	for _, c := range charts {
		chartIDs = append(chartIDs, c.ID)
		// charts to upsert - pair of filter, chart
		operation.SetFilter(bson.M{
			"_id": c.ID,
		})

		chartBSON, err := bson.Marshal(&c)
		var doc bson.M
		bson.Unmarshal(chartBSON, &doc)
		delete(doc, "_id")

		if err != nil {
			log.Errorf("Error marshalling chart to BSON: %v. Skipping this chart.", err)
		} else {
			update := doc
			operation.SetUpdate(update)
			operation.SetUpsert(true)
			operations = append(operations, operation)
		}
	}

	//Must use bulk write for array of filters
	collection := db.Collection(chartCollection)
	updateResult, err := collection.BulkWrite(
		context.Background(),
		operations,
		options.BulkWrite(),
	)

	//Set upsert flag and upsert the pairs here
	//Updates our index for charts that we already have and inserts charts that are new
	//updateResult, err := collection.UpdateMany(context.Background(), pairs, options.Update()..SetUpsert(true))
	log.Debugf("Chart import (upsert many) result: %v", updateResult)
	if err != nil {
		log.Errorf("Error occurred during chart import (upsert many). Err: %v", err)
		return err
	}
	log.Debugf("Upsert chart index success. %v documents inserted, %v documents upserted, %v documents modified", updateResult.InsertedCount, updateResult.UpsertedCount, updateResult.ModifiedCount)

	//Remove from our index, any charts that no longer exist
	filter := bson.M{
		"_id": bson.M{
			"$nin": chartIDs,
		},
		"repo.name": charts[0].Repo.Name,
	}
	deleteResult, err := collection.DeleteMany(context.Background(), filter, options.Delete())
	if err != nil {
		log.Errorf("Error occurred during chart import (delete many). Err: %v", err)
		return err
	}
	log.Debugf("Delete stale charts from index success. %v documents deleted.", deleteResult.DeletedCount)

	return err
}

func importWorker(db *mongo.Database, wg *sync.WaitGroup, icons <-chan types.Chart, chartFiles <-chan importChartFilesJob) {
	defer wg.Done()
	for c := range icons {
		log.WithFields(log.Fields{"name": c.Name}).Debug("importing icon")
		if err := fetchAndImportIcon(db, c); err != nil {
			log.WithFields(log.Fields{"name": c.Name}).WithError(err).Error("failed to import icon")
		}
	}
	for j := range chartFiles {
		log.WithFields(log.Fields{"name": j.Name, "version": j.ChartVersion.Version}).Debug("importing readme and values")
		if err := fetchAndImportFiles(db, j.Name, j.Repo, j.ChartVersion); err != nil {
			log.WithFields(log.Fields{"name": j.Name, "version": j.ChartVersion.Version}).WithError(err).Error("failed to import files")
		}
	}
}

func fetchAndImportIcon(db *mongo.Database, c types.Chart) error {
	if c.Icon == "" {
		log.WithFields(log.Fields{"name": c.Name}).Info("icon not found")
		return nil
	}

	req, err := http.NewRequest("GET", c.Icon, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", utils.UserAgent())
	if len(c.Repo.AuthorizationHeader) > 0 {
		req.Header.Set("Authorization", c.Repo.AuthorizationHeader)
	}

	res, err := netClient.Do(req)
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%d %s", res.StatusCode, c.Icon)
	}

	orig, err := imaging.Decode(res.Body)
	if err != nil {
		return err
	}

	// TODO: make this configurable?
	icon := imaging.Fit(orig, 160, 160, imaging.Lanczos)

	var b bytes.Buffer
	imaging.Encode(&b, icon, imaging.PNG)

	collection := db.Collection(chartCollection)
	//Update single icon
	update := bson.M{"$set": bson.M{"raw_icon": b.Bytes()}}
	filter := bson.M{"_id": c.ID}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update, options.Update())
	log.Debugf("Chart icon import (update one) result: %v", updateResult)
	if err != nil {
		log.Errorf("Error occurred during chart icon import (update one). Err: %v, Result: %v", err, updateResult)
		return err
	}
	return err
}

func fetchAndImportFiles(db *mongo.Database, name string, r types.Repo, cv types.ChartVersion) error {

	chartFilesID := fmt.Sprintf("%s/%s-%s", r.Name, name, cv.Version)
	//Check if we already have indexed files for this chart version and digest
	collection := db.Collection(chartFilesCollection)
	filter := bson.M{"_id": chartFilesID, "digest": cv.Digest}
	findResult := collection.FindOne(context.Background(), filter, options.FindOne())
	if findResult.Decode(&types.ChartFiles{}) != mongo.ErrNoDocuments {
		log.WithFields(log.Fields{"name": name, "version": cv.Version}).Debug("skipping existing files")
		return nil
	}
	log.WithFields(log.Fields{"name": name, "version": cv.Version}).Debug("fetching files")

	url := chartTarballURL(r, cv)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", utils.UserAgent())
	if len(r.AuthorizationHeader) > 0 {
		req.Header.Set("Authorization", r.AuthorizationHeader)
	}

	res, err := netClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// We read the whole chart into memory, this should be okay since the chart
	// tarball needs to be small enough to fit into a GRPC call (Tiller
	// requirement)
	gzf, err := gzip.NewReader(res.Body)
	if err != nil {
		return err
	}
	defer gzf.Close()

	tarf := tar.NewReader(gzf)

	readmeFileName := name + "/README.md"
	valuesFileName := name + "/values.yaml"
	filenames := []string{valuesFileName, readmeFileName}

	files, err := extractFilesFromTarball(filenames, tarf)
	if err != nil {
		return err
	}

	chartFiles := types.ChartFiles{ID: chartFilesID, Repo: r, Digest: cv.Digest}
	if v, ok := files[readmeFileName]; ok {
		chartFiles.Readme = v
	} else {
		log.WithFields(log.Fields{"name": name, "version": cv.Version}).Info("README.md not found")
	}
	if v, ok := files[valuesFileName]; ok {
		chartFiles.Values = v
		log.Debugf("CHart values: %v", v)
	} else {
		log.WithFields(log.Fields{"name": name, "version": cv.Version}).Info("values.yaml not found")
	}

	// inserts the chart files if not already indexed, or updates the existing
	// entry if digest has changed
	log.Debugf("Inserting file %v to collection: %v....", chartFilesID, chartFilesCollection)
	collection = db.Collection(chartFilesCollection)
	filter = bson.M{"_id": chartFilesID}
	chartBSON, err := bson.Marshal(&chartFiles)
	var doc bson.M
	bson.Unmarshal(chartBSON, &doc)
	delete(doc, "_id")
	update := bson.M{"$set": doc}
	updateResult, err := collection.UpdateOne(context.Background(), filter, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Errorf("Error occurred during chart files import (update one). Err: %v, Result: %v", err)
		return err
	}
	log.Debugf("Chart files import (update one) upserted: %v updated: %v", updateResult.UpsertedCount, updateResult.ModifiedCount)
	log.Debugf("Insert success.")
	return nil
}

func extractFilesFromTarball(filenames []string, tarf *tar.Reader) (map[string]string, error) {
	ret := make(map[string]string)
	for {
		header, err := tarf.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ret, err
		}

		for _, f := range filenames {
			if header.Name == f {
				var b bytes.Buffer
				io.Copy(&b, tarf)
				ret[f] = string(b.Bytes())
				break
			}
		}
	}
	return ret, nil
}

func chartTarballURL(r types.Repo, cv types.ChartVersion) string {
	source := cv.URLs[0]
	if _, err := parseRepoUrl(source); err != nil {
		// If the chart URL is not absolute, join with repo URL. It's fine if the
		// URL we build here is invalid as we can catch this error when actually
		// making the request
		u, _ := url.Parse(r.URL)
		u.Path = path.Join(u.Path, source)
		return u.String()
	}
	return source
}

func initNetClient(additionalCA string) (*http.Client, error) {
	// Get the SystemCertPool, continue with an empty pool on error
	caCertPool, _ := x509.SystemCertPool()
	if caCertPool == nil {
		caCertPool = x509.NewCertPool()
	}

	// If additionalCA exists, load it
	if _, err := os.Stat(additionalCA); !os.IsNotExist(err) {
		certs, err := ioutil.ReadFile(additionalCA)
		if err != nil {
			return nil, fmt.Errorf("Failed to append %s to RootCAs: %v", additionalCA, err)
		}

		// Append our cert to the system pool
		if ok := caCertPool.AppendCertsFromPEM(certs); !ok {
			return nil, fmt.Errorf("Failed to append %s to RootCAs", additionalCA)
		}
	}

	// Return Transport for testing purposes
	return &http.Client{
		Timeout: time.Second * defaultTimeoutSeconds,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}, nil
}
