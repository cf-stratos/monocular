package utils

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
