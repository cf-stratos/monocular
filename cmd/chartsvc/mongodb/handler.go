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

package mongodb

import (
	"fmt"
	"math"
	"net/http"

	"local/monocular/cmd/chartsvc/common"
	"local/monocular/cmd/chartsvc/models"

	"github.com/globalsign/mgo/bson"
	"github.com/gorilla/mux"
	"github.com/kubeapps/common/datastore"
	"github.com/kubeapps/common/response"
	log "github.com/sirupsen/logrus"
)

// Params a key-value map of path params
type Params map[string]string

// WithParams can be used to wrap handlers to take an extra arg for path params
type WithParams func(http.ResponseWriter, *http.Request, Params)

func (h WithParams) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	h(w, req, vars)
}

const chartCollection = "charts"
const filesCollection = "files"

// count is used to parse the result of a $count operation in the database
type count struct {
	Count int
}

var pathPrefix string
var dbSession datastore.Session

func SetPathPrefix(prefix string) {
	pathPrefix = prefix
}

func InitDBConfig(session datastore.Session, name string) {
	dbSession = session
}

func getPaginatedChartList(repo string, pageNumber, pageSize int) (common.ApiListResponse, interface{}, error) {
	log.Info("Request for charts..")
	db, closer := dbSession.DB()
	defer closer()
	var charts []*models.Chart

	c := db.C(chartCollection)
	pipeline := []bson.M{}
	if repo != "" {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"repo.name": repo}})
	}

	// We should query unique charts
	pipeline = append(pipeline,
		// Add a new field to store the latest version
		bson.M{"$addFields": bson.M{"firstChartVersion": bson.M{"$arrayElemAt": []interface{}{"$chartversions", 0}}}},
		// Group by unique digest for the latest version (remove duplicates)
		bson.M{"$group": bson.M{"_id": "$firstChartVersion.digest", "chart": bson.M{"$first": "$$ROOT"}}},
		// Restore original object struct
		bson.M{"$replaceRoot": bson.M{"newRoot": "$chart"}},
		// Order by name
		bson.M{"$sort": bson.M{"name": 1}},
	)

	totalPages := 1
	if pageSize != 0 {
		// If a pageSize is given, returns only the the specified number of charts and
		// the number of pages
		countPipeline := append(pipeline, bson.M{"$count": "count"})
		cc := count{}
		err := c.Pipe(countPipeline).One(&cc)
		if err != nil {
			return common.ApiListResponse{}, 0, err
		}
		totalPages = int(math.Ceil(float64(cc.Count) / float64(pageSize)))

		// If the page number is out of range, return the last one
		if pageNumber > totalPages {
			pageNumber = totalPages
		}

		pipeline = append(pipeline,
			bson.M{"$skip": pageSize * (pageNumber - 1)},
			bson.M{"$limit": pageSize},
		)
	}
	err := c.Pipe(pipeline).All(&charts)
	if err != nil {
		return common.ApiListResponse{}, 0, err
	}
	log.Infof("Done. Returning %v charts.", len(charts))
	return common.NewChartListResponse(charts, pathPrefix), common.Meta{totalPages}, nil
}

// listCharts returns a list of charts
func ListCharts(w http.ResponseWriter, req *http.Request) {
	log.Info("Request for charts..")
	pageNumber, pageSize := common.GetPageNumberAndSize(req)
	cl, meta, err := getPaginatedChartList("", pageNumber, pageSize)
	if err != nil {
		log.WithError(err).Error("could not fetch charts")
		response.NewErrorResponse(http.StatusInternalServerError, "could not fetch all charts").Write(w)
		return
	}
	response.NewDataResponseWithMeta(cl, meta).Write(w)
	log.Info("Done.")
}

// listRepoCharts returns a list of charts in the given repo
func ListRepoCharts(w http.ResponseWriter, req *http.Request, params Params) {
	log.Info("Request for charts..")
	pageNumber, pageSize := common.GetPageNumberAndSize(req)
	cl, meta, err := getPaginatedChartList(params["repo"], pageNumber, pageSize)
	if err != nil {
		log.WithError(err).Error("could not fetch charts")
		response.NewErrorResponse(http.StatusInternalServerError, "could not fetch all charts").Write(w)
		return
	}
	response.NewDataResponseWithMeta(cl, meta).Write(w)
	log.Info("Done.")
}

// getChart returns the chart from the given repo
func GetChart(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var chart models.Chart
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	if err := db.C(chartCollection).FindId(chartID).One(&chart); err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart").Write(w)
		return
	}

	cr := common.NewChartResponse(&chart, pathPrefix)
	response.NewDataResponse(cr).Write(w)
}

// listChartVersions returns a list of chart versions for the given chart
func ListChartVersions(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var chart models.Chart
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	if err := db.C(chartCollection).FindId(chartID).One(&chart); err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart").Write(w)
		return
	}

	cvl := common.NewChartVersionListResponse(&chart, pathPrefix)
	response.NewDataResponse(cvl).Write(w)
}

// getChartVersion returns the given chart version
func GetChartVersion(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var chart models.Chart
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	if err := db.C(chartCollection).Find(bson.M{
		"_id":           chartID,
		"chartversions": bson.M{"$elemMatch": bson.M{"version": params["version"]}},
	}).Select(bson.M{
		"name": 1, "repo": 1, "description": 1, "home": 1, "keywords": 1, "maintainers": 1, "sources": 1,
		"chartversions.$": 1,
	}).One(&chart); err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		response.NewErrorResponse(http.StatusNotFound, "could not find chart version").Write(w)
		return
	}

	cvr := common.NewChartVersionResponse(&chart, chart.ChartVersions[0], pathPrefix)
	response.NewDataResponse(cvr).Write(w)
}

// getChartIcon returns the icon for a given chart
func GetChartIcon(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var chart models.Chart
	chartID := fmt.Sprintf("%s/%s", params["repo"], params["chartName"])
	if err := db.C(chartCollection).FindId(chartID).One(&chart); err != nil {
		log.WithError(err).Errorf("could not find chart with id %s", chartID)
		http.NotFound(w, req)
		return
	}

	if chart.RawIcon == nil {
		http.NotFound(w, req)
		return
	}

	w.Write(chart.RawIcon)
}

// getChartVersionReadme returns the README for a given chart
func GetChartVersionReadme(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var files models.ChartFiles
	fileID := fmt.Sprintf("%s/%s-%s", params["repo"], params["chartName"], params["version"])
	if err := db.C(filesCollection).FindId(fileID).One(&files); err != nil {
		log.WithError(err).Errorf("could not find files with id %s", fileID)
		http.NotFound(w, req)
		return
	}
	readme := []byte(files.Readme)
	if len(readme) == 0 {
		log.Errorf("could not find a README for id %s", fileID)
		http.NotFound(w, req)
		return
	}
	w.Write(readme)
}

// getChartVersionValues returns the values.yaml for a given chart
func GetChartVersionValues(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()
	var files models.ChartFiles
	fileID := fmt.Sprintf("%s/%s-%s", params["repo"], params["chartName"], params["version"])
	if err := db.C(filesCollection).FindId(fileID).One(&files); err != nil {
		log.WithError(err).Errorf("could not find values.yaml with id %s", fileID)
		http.NotFound(w, req)
		return
	}

	w.Write([]byte(files.Values))
}

// listChartsWithFilters returns the list of repos that contains the given chart and the latest version found
func ListChartsWithFilters(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()

	var charts []*models.Chart
	if err := db.C(chartCollection).Find(bson.M{
		"name": params["chartName"],
		"chartversions": bson.M{
			"$elemMatch": bson.M{"version": req.FormValue("version"), "appversion": req.FormValue("appversion")},
		}}).Select(bson.M{
		"name": 1, "repo": 1,
		"chartversions": bson.M{"$slice": 1},
	}).All(&charts); err != nil {
		log.WithError(err).Errorf(
			"could not find charts with the given name %s, version %s and appversion %s",
			params["chartName"], req.FormValue("version"), req.FormValue("appversion"),
		)
		// continue to return empty list
	}

	cl := common.NewChartListResponse(common.UniqChartList(charts), pathPrefix)
	response.NewDataResponse(cl).Write(w)
}

// searchCharts returns the list of charts that matches the query param in any of these fields:
//  - name
//  - description
//  - repository name
//  - any keyword
//  - any source
//  - any maintainer name
func SearchCharts(w http.ResponseWriter, req *http.Request, params Params) {
	db, closer := dbSession.DB()
	defer closer()

	query := req.FormValue("q")
	var charts []*models.Chart
	conditions := bson.M{
		"$or": []bson.M{
			{"name": bson.M{"$regex": query}},
			{"description": bson.M{"$regex": query}},
			{"repo.name": bson.M{"$regex": query}},
			{"keywords": bson.M{"$elemMatch": bson.M{"$regex": query}}},
			{"sources": bson.M{"$elemMatch": bson.M{"$regex": query}}},
			{"maintainers": bson.M{"$elemMatch": bson.M{"name": bson.M{"$regex": query}}}},
		},
	}
	if params["repo"] != "" {
		conditions["repo.name"] = params["repo"]
	}
	if err := db.C(chartCollection).Find(conditions).All(&charts); err != nil {
		log.WithError(err).Errorf(
			"could not find charts with the given query %s",
			query,
		)
		// continue to return empty list
	}

	cl := common.NewChartListResponse(common.UniqChartList(charts), pathPrefix)
	response.NewDataResponse(cl).Write(w)
}
