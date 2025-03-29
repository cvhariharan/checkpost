package handlers

import (
	"github.com/cvhariharan/watcher/internal/models"
)

type Machine struct {
	ID       string
	Hostname string
	Platform string
	Status   string
	LastSeen string
	Tags     []string
}

type Query struct {
	ID          string
	Name        string
	Description string
	SQL         string
	LastRun     string
}

type Pack struct {
	ID          string
	Name        string
	Description string
	Queries     int
	Targets     int
}

type Schedule struct {
	ID       string
	Name     string
	Query    string
	Interval string
	LastRun  string
	NextRun  string
}

type IndexPageData struct {
	Title        string
	Active       string
	Machines     []Machine
	Queries      []Query
	Packs        []Pack
	Schedules    []Schedule
	AllTags      []string
	ErrorCode    int
	ErrorMessage string
}

type EnrollmentRequest struct {
	EnrollSecret   string `json:"enroll_secret"`
	HostIdentifier string `json:"host_identifier"`

	HostDetails models.HostDetailsInfo `json:"host_details"`
}

func (e EnrollmentRequest) ToNodeModel() models.Node {
	return models.Node{
		HostIdentifier: e.HostIdentifier,
		OSVersion:      e.HostDetails.OSVersion,
		OSQuery:        e.HostDetails.OSQuery,
		Platform:       e.HostDetails.Platform,
		System:         e.HostDetails.System,
	}
}

type EnrollmentResponse struct {
	NodeKey     string `json:"node_key"`
	NodeInvalid bool   `json:"node_invalid"`
}

type CreateQueryRequest struct {
	Query       string `json:"query"`
	Description string `json:"description"`
}

type CreateResponse struct {
	ID string `json:"string"`
}

type PaginateRequest struct {
	Page  int `query:"page"`
	Count int `query:"count_per_page"`
}

type PaginateQueriesResponse struct {
	Queries    []models.Query `json:"queries"`
	TotalCount int            `json:"total_count"`
	PageCount  int            `json:"page_count"`
}

type GetRequest struct {
	ID string `param:"id"`
}

type UpdateQueryRequest struct {
	ID          string `param:"id"`
	Query       string `json:"query"`
	Description string `json:"description"`
}

type CreateScheduleRequest struct {
	QueryID  string `json:"query_id" validate:"required,uuid4"`
	Title    string `json:"title" validate:"required,ascii"`
	Interval int    `json:"interval" validate:"required,lte=604800"`
	Removed  bool   `json:"removed"`
	Snapshot bool   `json:"snapshot"`
	Platform string `json:"platform" validate:"oneof=darwin linux posiz windows any all"`
	Version  string `json:"version"`
	Shard    int    `json:"shard" validate:"lte=100"`
	Denylist bool   `json:"denylist"`
}
