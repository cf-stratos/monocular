package common

import (
	"fmt"
	"local/monocular/cmd/chartsvc/models"
)

type BodyAPIListResponse struct {
	Data *ApiListResponse `json:"data"`
	Meta Meta             `json:"meta,omitempty"`
}

type BodyAPIResponse struct {
	Data ApiResponse `json:"data"`
}

type ApiResponse struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Attributes    interface{} `json:"attributes"`
	Links         interface{} `json:"links"`
	Relationships RelMap      `json:"relationships"`
}

type ApiListResponse []*ApiResponse

func NewChartResponse(c *models.Chart, pathPrefix string) *ApiResponse {
	latestCV := c.ChartVersions[0]
	return &ApiResponse{
		Type:       "chart",
		ID:         c.ID,
		Attributes: ChartAttributes(*c, pathPrefix),
		Links:      SelfLink{pathPrefix + "/charts/" + c.ID},
		Relationships: RelMap{
			"latestChartVersion": Rel{
				Data:  ChartVersionAttributes(c.ID, latestCV, pathPrefix),
				Links: SelfLink{pathPrefix + "/charts/" + c.ID + "/versions/" + latestCV.Version},
			},
		},
	}
}

func NewChartListResponse(charts []*models.Chart, pathPrefix string) ApiListResponse {
	cl := ApiListResponse{}
	for _, c := range charts {
		cl = append(cl, NewChartResponse(c, pathPrefix))
	}
	return cl
}

func NewChartVersionResponse(c *models.Chart, cv models.ChartVersion, pathPrefix string) *ApiResponse {
	return &ApiResponse{
		Type:       "chartVersion",
		ID:         fmt.Sprintf("%s-%s", c.ID, cv.Version),
		Attributes: ChartVersionAttributes(c.ID, cv, pathPrefix),
		Links:      SelfLink{pathPrefix + "/charts/" + c.ID + "/versions/" + cv.Version},
		Relationships: RelMap{
			"chart": Rel{
				Data:  ChartAttributes(*c, pathPrefix),
				Links: SelfLink{pathPrefix + "/charts/" + c.ID},
			},
		},
	}
}

func NewChartVersionListResponse(c *models.Chart, pathPrefix string) ApiListResponse {
	var cvl ApiListResponse
	for _, cv := range c.ChartVersions {
		cvl = append(cvl, NewChartVersionResponse(c, cv, pathPrefix))
	}

	return cvl
}
