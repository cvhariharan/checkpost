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
	Schedules    []Schedule
	AllTags      []string
	ErrorCode    int
	ErrorMessage string
}

type EnrollmentRequest struct {
	EnrollSecret   string `json:"enroll_secret" validate:"required"`
	HostIdentifier string `json:"host_identifier" validate:"required"`

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

type CreateResponse struct {
	ID string `json:"id"`
}

type PaginateRequest struct {
	Page  int `query:"page" validate:"gte=0"`
	Count int `query:"count_per_page" validate:"gte=0"`
}

type MachineListRequest struct {
	Page     int    `query:"page" validate:"gte=0"`
	Count    int    `query:"count_per_page" validate:"gte=0"`
	Query    string `query:"q" validate:"lte=4096"`
	Platform string `query:"platform" validate:"lte=255"`
	OwnerID  string `query:"owner_id" validate:"omitempty,uuid"`
	Assigned string `query:"assigned" validate:"omitempty,oneof=assigned unassigned"`
}

type PaginateMachinesResponse struct {
	Machines   []models.Node `json:"machines"`
	TotalCount int           `json:"total_count"`
	PageCount  int           `json:"page_count"`
}

type GetRequest struct {
	ID string `param:"id" validate:"required,uuid"`
}

type MachineQueryRequest struct {
	ID    string `param:"id" validate:"required,uuid"`
	Query string `json:"query" validate:"required"`
}

type MachineQueriesRequest struct {
	ID    string `param:"id" validate:"required,uuid"`
	Page  int    `query:"page" validate:"gte=0"`
	Count int    `query:"count_per_page" validate:"gte=0"`
}

type DeleteMachineQueryRequest struct {
	ID      string `param:"id" validate:"required,uuid"`
	QueryID string `param:"query_id" validate:"required,uuid"`
}

type MachineQueriesResponse struct {
	Queries    []models.MachineQueryResult `json:"queries"`
	TotalCount int                         `json:"total_count"`
	PageCount  int                         `json:"page_count"`
}

type CreateScheduleRequest struct {
	Query       string   `json:"query" validate:"required"`
	Description string   `json:"description"`
	Title       string   `json:"title" validate:"required,ascii"`
	Interval    int      `json:"interval" validate:"gte=1,lte=604800"`
	Removed     bool     `json:"removed"`
	Snapshot    bool     `json:"snapshot"`
	Platform    string   `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Version     string   `json:"version"`
	Shard       int      `json:"shard" validate:"gte=0,lte=100"`
	Denylist    bool     `json:"denylist"`
	GroupIDs    []string `json:"group_ids" validate:"omitempty,dive,uuid"`
}

type PaginateSchedulesResponse struct {
	Schedules  []models.Schedule `json:"schedules"`
	TotalCount int               `json:"total_count"`
	PageCount  int               `json:"page_count"`
}

type CreatePolicyRequest struct {
	Title       string   `json:"title" validate:"required"`
	Query       string   `json:"query" validate:"required"`
	Description string   `json:"description"`
	Resolution  string   `json:"resolution"`
	Platform    string   `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Enabled     *bool    `json:"enabled"`
	GroupIDs    []string `json:"group_ids" validate:"omitempty,dive,uuid"`
}

type UpdatePolicyRequest struct {
	ID          string   `param:"id" validate:"required,uuid"`
	Title       string   `json:"title" validate:"required"`
	Query       string   `json:"query" validate:"required"`
	Description string   `json:"description"`
	Resolution  string   `json:"resolution"`
	Platform    string   `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Enabled     *bool    `json:"enabled"`
	GroupIDs    []string `json:"group_ids" validate:"omitempty,dive,uuid"`
}

type PaginatePoliciesResponse struct {
	Policies   []models.Policy `json:"policies"`
	TotalCount int             `json:"total_count"`
	PageCount  int             `json:"page_count"`
}

type PolicyMachinesRequest struct {
	ID       string `param:"id" validate:"required,uuid"`
	Response string `query:"response" validate:"omitempty,oneof=passing failing unknown"`
	Page     int    `query:"page" validate:"gte=0"`
	Count    int    `query:"count_per_page" validate:"gte=0"`
}

type PolicyMachinesResponse struct {
	Machines   []models.PolicyMachine `json:"machines"`
	TotalCount int                    `json:"total_count"`
	PageCount  int                    `json:"page_count"`
}

type CreateGroupRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
}

type UpdateGroupRequest struct {
	ID          string `param:"id" validate:"required,uuid"`
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
}

type PaginateGroupsResponse struct {
	Groups     []models.Group `json:"groups"`
	TotalCount int            `json:"total_count"`
	PageCount  int            `json:"page_count"`
}

type MachineGroupsRequest struct {
	ID string `param:"id" validate:"required,uuid"`
}

type ReplaceMachineGroupsRequest struct {
	ID       string   `param:"id" validate:"required,uuid"`
	GroupIDs []string `json:"group_ids" validate:"omitempty,dive,uuid"`
}

type MachineGroupsResponse struct {
	Groups []models.Group `json:"groups"`
}

type GroupMachinesRequest struct {
	ID    string `param:"id" validate:"required,uuid"`
	Page  int    `query:"page" validate:"gte=0"`
	Count int    `query:"count_per_page" validate:"gte=0"`
}

type GroupMachinesResponse struct {
	Machines   []models.Node `json:"machines"`
	TotalCount int           `json:"total_count"`
	PageCount  int           `json:"page_count"`
}

type OwnersRequest struct {
	Page  int    `query:"page" validate:"gte=0"`
	Count int    `query:"count_per_page" validate:"gte=0"`
	Query string `query:"q" validate:"lte=4096"`
}

type CreateOwnerRequest struct {
	DisplayName string `json:"display_name" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	ExternalID  string `json:"external_id"`
	Department  string `json:"department"`
	Title       string `json:"title"`
	Phone       string `json:"phone"`
	Notes       string `json:"notes"`
}

type UpdateOwnerRequest struct {
	ID          string `param:"id" validate:"required,uuid"`
	DisplayName string `json:"display_name" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	ExternalID  string `json:"external_id"`
	Department  string `json:"department"`
	Title       string `json:"title"`
	Phone       string `json:"phone"`
	Notes       string `json:"notes"`
}

type PaginateOwnersResponse struct {
	Owners     []models.DeviceOwner `json:"owners"`
	TotalCount int                  `json:"total_count"`
	PageCount  int                  `json:"page_count"`
}

type OwnerMachinesRequest struct {
	ID    string `param:"id" validate:"required,uuid"`
	Page  int    `query:"page" validate:"gte=0"`
	Count int    `query:"count_per_page" validate:"gte=0"`
}

type OwnerMachinesResponse struct {
	Machines   []models.Node `json:"machines"`
	TotalCount int           `json:"total_count"`
	PageCount  int           `json:"page_count"`
}

type UpdateMachineInventoryRequest struct {
	ID                 string  `param:"id" validate:"required,uuid"`
	OwnerID            *string `json:"owner_id" validate:"omitempty,uuid"`
	InternalTrackingID string  `json:"internal_tracking_id"`
	Notes              string  `json:"notes"`
}

type MachineInventoryResponse struct {
	Inventory models.NodeInventory `json:"inventory"`
}

type PatchGroupMachinesRequest struct {
	ID            string   `param:"id" validate:"required,uuid"`
	AddNodeIDs    []string `json:"add_node_ids" validate:"omitempty,dive,uuid"`
	RemoveNodeIDs []string `json:"remove_node_ids" validate:"omitempty,dive,uuid"`
}

type UpdateScheduleRequest struct {
	ID            string   `param:"id" validate:"required,uuid"`
	Query         string   `json:"query" validate:"required"`
	Description   string   `json:"description"`
	Title         string   `json:"title" validate:"required,ascii"`
	Interval      int      `json:"interval" validate:"gte=1,lte=604800"`
	Removed       bool     `json:"removed"`
	Snapshot      bool     `json:"snapshot"`
	Platform      string   `json:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Version       string   `json:"version"`
	Shard         int      `json:"shard" validate:"gte=0,lte=100"`
	Denylist      bool     `json:"denylist"`
	RetentionDays int      `json:"retention_days" validate:"omitempty,gte=1,lte=365"`
	GroupIDs      []string `json:"group_ids" validate:"omitempty,dive,uuid"`
}

type ScheduleResultsRequest struct {
	ID    string `param:"id" validate:"required,uuid"`
	Page  int    `query:"page" validate:"gte=0"`
	Count int    `query:"count_per_page" validate:"gte=0,lte=1000"`
	Query string `query:"q" validate:"lte=4096"`
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
	NodeKey string                   `json:"node_key" validate:"required,uuid"`
	Data    []map[string]interface{} `json:"data" validate:"required"`
	LogType string                   `json:"log_type" validate:"required,oneof=result status"`
}

type DistributedReadRequest struct {
	NodeKey string `json:"node_key" validate:"required,uuid"`
}

type DistributedReadResponse struct {
	Queries map[string]string `json:"queries"`
}

type DistributedWriteRequest struct {
	NodeKey  string                   `json:"node_key" validate:"required,uuid"`
	Queries  map[string]interface{}   `json:"queries" validate:"required"`
	Statuses map[string]OsqueryStatus `json:"statuses"`
	Messages map[string]string        `json:"messages"`
}
