package handlers

import (
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/watcher/internal/models"
)

type OsqueryStatus string

func (s *OsqueryStatus) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var v string
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		*s = OsqueryStatus(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("osquery status must be a string or number: %w", err)
	}
	*s = OsqueryStatus(n.String())
	return nil
}

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

func (e EnrollmentRequest) ToNodeModel() models.NodeEnrollment {
	return models.NodeEnrollment{
		HostIdentifier: e.HostIdentifier,
		HostDetails:    e.HostDetails,
	}
}

type EnrollmentResponse struct {
	NodeKey     string `json:"node_key"`
	NodeInvalid bool   `json:"node_invalid"`
}

type CreateQueryRequest struct {
	Title       string `json:"title"`
	Query       string `json:"query"`
	Description string `json:"description"`
}

type CreateResponse struct {
	ID string `json:"id"`
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

type PaginateMachinesResponse struct {
	Machines   []models.Node `json:"machines"`
	TotalCount int           `json:"total_count"`
	PageCount  int           `json:"page_count"`
}

type GetRequest struct {
	ID string `param:"id"`
}

type MachineQueryRequest struct {
	ID    string `param:"id"`
	Query string `json:"query"`
}

type MachineQueriesRequest struct {
	ID    string `param:"id"`
	Page  int    `query:"page"`
	Count int    `query:"count_per_page"`
}

type DeleteMachineQueryRequest struct {
	ID      string `param:"id"`
	QueryID string `param:"query_id"`
}

type MachineQueriesResponse struct {
	Queries    []models.MachineQueryResult `json:"queries"`
	TotalCount int                         `json:"total_count"`
	PageCount  int                         `json:"page_count"`
}

type UpdateQueryRequest struct {
	ID          string `param:"id"`
	Title       string `json:"title"`
	Query       string `json:"query"`
	Description string `json:"description"`
}

type CreateScheduleRequest struct {
	QueryID  string `json:"query_id" validate:"required,uuid"`
	Title    string `json:"title" validate:"required,ascii"`
	Interval int    `json:"interval" validate:"required,lte=604800"`
	Removed  bool   `json:"removed"`
	Snapshot bool   `json:"snapshot"`
	Platform string `json:"platform" validate:"oneof=darwin linux posix windows any all"`
	Version  string `json:"version"`
	Shard    int    `json:"shard" validate:"lte=100"`
	Denylist bool   `json:"denylist"`
}

type PaginateSchedulesResponse struct {
	Schedules  []models.Schedule `json:"schedules"`
	TotalCount int               `json:"total_count"`
	PageCount  int               `json:"page_count"`
}

type CreatePolicyRequest struct {
	Title       string `json:"title" validate:"required"`
	Query       string `json:"query" validate:"required"`
	Description string `json:"description"`
	Resolution  string `json:"resolution"`
	Platform    string `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Enabled     *bool  `json:"enabled"`
}

type UpdatePolicyRequest struct {
	ID          string `param:"id"`
	Title       string `json:"title" validate:"required"`
	Query       string `json:"query" validate:"required"`
	Description string `json:"description"`
	Resolution  string `json:"resolution"`
	Platform    string `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Enabled     *bool  `json:"enabled"`
}

type PaginatePoliciesResponse struct {
	Policies   []models.Policy `json:"policies"`
	TotalCount int             `json:"total_count"`
	PageCount  int             `json:"page_count"`
}

type PolicyMachinesRequest struct {
	ID       string `param:"id"`
	Response string `query:"response"`
	Page     int    `query:"page"`
	Count    int    `query:"count_per_page"`
}

type PolicyMachinesResponse struct {
	Machines   []models.PolicyMachine `json:"machines"`
	TotalCount int                    `json:"total_count"`
	PageCount  int                    `json:"page_count"`
}

type UpdateScheduleRequest struct {
	ID       string `param:"id"`
	QueryID  string `json:"query_id" validate:"required,uuid"`
	Title    string `json:"title" validate:"required,ascii"`
	Interval int    `json:"interval" validate:"required,lte=604800"`
	Removed  bool   `json:"removed"`
	Snapshot bool   `json:"snapshot"`
	Platform string `json:"platform" validate:"oneof=darwin linux posix windows any all"`
	Version  string `json:"version"`
	Shard    int    `json:"shard" validate:"lte=100"`
	Denylist bool   `json:"denylist"`
}

type ConfigRequest struct {
	NodeKey string `json:"node_key" validate:"required,uuid"`
}

type ScheduleConfig struct {
	Query    string `json:"query"`
	Interval int    `json:"interval"`
	Platform string `json:"platform"`
	Snapshot bool   `json:"snapshot"`
}

type OSQueryConfigResponse struct {
	Schedule map[string]ScheduleConfig `json:"schedule"`
}

type LogRequest struct {
	NodeKey string                   `json:"node_key"`
	Data    []map[string]interface{} `json:"data"`
	LogType string                   `json:"log_type"`
}

type DistributedReadRequest struct {
	NodeKey string `json:"node_key" validate:"required,uuid"`
}

type DistributedReadResponse struct {
	Queries map[string]string `json:"queries"`
}

type DistributedWriteRequest struct {
	NodeKey  string                   `json:"node_key" validate:"required,uuid"`
	Queries  map[string]interface{}   `json:"queries"`
	Statuses map[string]OsqueryStatus `json:"statuses"`
	Messages map[string]string        `json:"messages"`
}
