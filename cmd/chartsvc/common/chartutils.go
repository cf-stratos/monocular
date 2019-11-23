package common

import (
	"local/monocular/cmd/chartsvc/models"
	"net/http"
	"strconv"
)

type SelfLink struct {
	Self string `json:"self"`
}

type RelMap map[string]Rel

type Rel struct {
	Data  interface{} `json:"data"`
	Links SelfLink    `json:"links"`
}

type Meta struct {
	TotalPages int `json:"totalPages"`
}

func chartVersionAttributes(cid string, cv models.ChartVersion, pathPrefix string) models.ChartVersion {
	cv.Readme = pathPrefix + "/assets/" + cid + "/versions/" + cv.Version + "/README.md"
	cv.Values = pathPrefix + "/assets/" + cid + "/versions/" + cv.Version + "/values.yaml"
	return cv
}

func chartAttributes(c models.Chart, pathPrefix string) models.Chart {
	if c.RawIcon != nil {
		c.Icon = pathPrefix + "/assets/" + c.ID + "/logo-160x160-fit.png"
	} else {
		// If the icon wasn't processed, it is either not set or invalid
		c.Icon = ""
	}
	return c
}

func GetPageNumberAndSize(req *http.Request) (int, int) {
	page := req.FormValue("page")
	size := req.FormValue("size")
	pageInt, err := strconv.ParseUint(page, 10, 64)
	if err != nil {
		pageInt = 1
	}
	// ParseUint will return 0 if size is a not positive integer
	sizeInt, _ := strconv.ParseUint(size, 10, 64)
	return int(pageInt), int(sizeInt)
}

// min returns the minimum of two integers.
// We are not using math.Min since that compares float64
// and it's unnecessarily complex.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func UniqChartList(charts []*models.Chart) []*models.Chart {
	// We will keep track of unique digest:chart to avoid duplicates
	chartDigests := map[string]bool{}
	res := []*models.Chart{}
	for _, c := range charts {
		digest := c.ChartVersions[0].Digest
		// Filter out the chart if we've seen the same digest before
		if _, ok := chartDigests[digest]; !ok {
			chartDigests[digest] = true
			res = append(res, c)
		}
	}
	return res
}
