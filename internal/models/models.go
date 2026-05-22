package models

import "time"

type Node struct {
	ID              int64           `json:"-"`
	ResourceID      string          `json:"id"`
	UUID            string          `json:"uuid"`
	NodeKey         string          `json:"node_key"`
	HostIdentifier  string          `json:"host_identifier"`
	Hostname        string          `json:"hostname"`
	Platform        string          `json:"platform"`
	OSName          string          `json:"os_name"`
	OSVersion       string          `json:"os_version"`
	OSQueryVersion  string          `json:"osquery_version"`
	HardwareSerial  string          `json:"hardware_serial"`
	EnrolledAt      time.Time       `json:"enrolled_at"`
	LastSeenAt      *time.Time      `json:"last_seen_at,omitempty"`
	PolicyUpdatedAt *time.Time      `json:"policy_updated_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	EnrollmentInput HostDetailsInfo `json:"-"`
}

type NodeEnrollment struct {
	HostIdentifier string
	HostDetails    HostDetailsInfo
}

type NodeCredentials struct {
	NodeKey string `json:"node_key"`
}

type OSVersionInfo struct {
	UUID         string `json:"uuid"`
	OSID         string `json:"os_id"`
	Codename     string `json:"codename"`
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	Name         string `json:"name"`
	Patch        string `json:"patch"`
	Platform     string `json:"platform"`
	PlatformLike string `json:"platform_like"`
	Version      string `json:"version"`
	NodeFK       int    `json:"-"`
}

type OsqueryInfo struct {
	UUID          string `json:"uuid"`
	BuildDistro   string `json:"build_distro"`
	BuildPlatform string `json:"build_platform"`
	ConfigHash    string `json:"config_hash"`
	ConfigValid   string `json:"config_valid"`
	Extension     string `json:"extensions"`
	InstanceID    string `json:"instance_id"`
	PID           string `json:"pid"`
	StartTime     string `json:"start_time"`
	Version       string `json:"version"`
	Watcher       string `json:"watcher"`
	NodeFK        int    `json:"-"`
}

type SystemInfo struct {
	UUID             string `json:"uuid"`
	ComputerName     string `json:"computer_name"`
	CPUBrand         string `json:"cpu_brand"`
	CPULogicalCores  string `json:"cpu_logical_cores"`
	CPUPhysicalCores string `json:"cpu_physical_cores"`
	CPUSubtype       string `json:"cpu_subtype"`
	CPUType          string `json:"cpu_type"`
	HardwareModel    string `json:"hardware_model"`
	HardwareSerial   string `json:"hardware_serial"`
	HardwareVendor   string `json:"hardware_vendor"`
	HardwareVersion  string `json:"hardware_version"`
	Hostname         string `json:"hostname"`
	LocalHostname    string `json:"local_hostname"`
	PhysicalMemory   string `json:"physical_memory"`
	NodeFK           int    `json:"-"`
}

type PlatformInfo struct {
	UUID       string `json:"uuid"`
	Address    string `json:"address"`
	Date       string `json:"date"`
	Extra      string `json:"extra"`
	Revision   string `json:"revision"`
	Size       string `json:"size"`
	Vendor     string `json:"vendor"`
	Version    string `json:"version"`
	VolumeSize string `json:"volume_size"`
	NodeFK     int    `json:"-"`
}

type HostDetailsInfo struct {
	OSVersion OSVersionInfo `json:"os_version"`
	OSQuery   OsqueryInfo   `json:"osquery_info"`
	System    SystemInfo    `json:"system_info"`
	Platform  PlatformInfo  `json:"platform_info"`
}

type Query struct {
	ID          int64     `json:"-"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	SQL         string    `json:"sql"`
	Title       string    `json:"title"`
	Query       string    `json:"query"`
	IsSystem    bool      `json:"is_system"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateQuery struct {
	Name        string
	SQL         string
	Description string
	IsSystem    bool
}

type UpdateQuery struct {
	UUID        string
	Name        string
	SQL         string
	Description string
}

type Schedule struct {
	ID              int64     `json:"-"`
	UUID            string    `json:"uuid"`
	QueryID         string    `json:"query_id"`
	Query           Query     `json:"query"`
	Name            string    `json:"name"`
	Title           string    `json:"title"`
	IntervalSeconds int       `json:"interval_seconds"`
	Interval        int       `json:"interval"`
	Removed         bool      `json:"removed"`
	Snapshot        bool      `json:"snapshot"`
	Platform        string    `json:"platform"`
	Version         string    `json:"version"`
	Shard           int       `json:"shard"`
	Enabled         bool      `json:"enabled"`
	IsSystem        bool      `json:"is_system"`
	Denylist        bool      `json:"denylist"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateSchedule struct {
	QueryUUID       string
	Name            string
	IntervalSeconds int
	Removed         bool
	Snapshot        bool
	Platform        string
	Version         string
	Shard           int
	Denylist        bool
	Enabled         bool
	IsSystem        bool
}

type UpdateSchedule struct {
	UUID            string
	QueryUUID       string
	Name            string
	IntervalSeconds int
	Removed         bool
	Snapshot        bool
	Platform        string
	Version         string
	Shard           int
	Denylist        bool
	Enabled         bool
}

type Policy struct {
	ID                 int64      `json:"-"`
	ResourceID         string     `json:"id"`
	UUID               string     `json:"uuid"`
	Name               string     `json:"name"`
	Title              string     `json:"title"`
	Query              string     `json:"query"`
	Description        string     `json:"description"`
	Resolution         string     `json:"resolution"`
	Platform           string     `json:"platform"`
	Enabled            bool       `json:"enabled"`
	IsSystem           bool       `json:"is_system"`
	PassingCount       int        `json:"passing_count"`
	FailingCount       int        `json:"failing_count"`
	UnknownCount       int        `json:"unknown_count"`
	LastCountUpdatedAt *time.Time `json:"last_count_updated_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreatePolicy struct {
	Name        string
	Query       string
	Description string
	Resolution  string
	Platform    string
	Enabled     bool
	IsSystem    bool
}

type UpdatePolicy struct {
	UUID        string
	Name        string
	Query       string
	Description string
	Resolution  string
	Platform    string
	Enabled     bool
}

type PolicyPosture struct {
	Policy
	Response  string     `json:"response"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
	Stale     bool       `json:"stale"`
}

type PolicyMachine struct {
	Node
	Response  string     `json:"response"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
	Stale     bool       `json:"stale"`
}

type PolicyMachinesRequest struct {
	PolicyUUID string
	Response   string
	Page       int
	Count      int
}

type PageRequest struct {
	Page  int
	Count int
}

type Page[T any] struct {
	Items      []T
	TotalCount int
	PageCount  int
}

type ResourceID struct {
	UUID string
}

type NodeIdentity struct {
	ID string
}

type MachineQueryRequest struct {
	NodeUUID string
	Query    string
}

type MachineQueryResult struct {
	ID        string      `json:"id"`
	Query     string      `json:"query"`
	Status    string      `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
	Results   interface{} `json:"results,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type NodeKeyRequest struct {
	NodeKey string
}

type ScheduleListRequest struct {
	Limit int
}

type OsqueryLogBatch struct {
	NodeKey string
	LogType string
	Data    []map[string]interface{}
}

type OsqueryResultBatch struct {
	NodeID       int64
	ScheduleName string
	Action       string
	CalendarTime string
	Counter      int64
	Epoch        int64
	Numerics     bool
	UnixTime     time.Time
	IsSystem     bool
	Rows         []OsqueryResultRow
}

type OsqueryResultRow map[string]string

type OsqueryStatusLog struct {
	NodeID       int64
	CalendarTime string
	FileName     string
	Line         int32
	Message      string
	Severity     int32
	UnixTime     time.Time
	Version      string
}

type Pack struct {
	UUID      string   `json:"uuid"`
	Discovery []string `json:"discovery"`
	Platform  string   `json:"platform"`
	Version   string   `json:"version"`
	Shard     int      `json:"shard"`
}

type Packs map[string]Pack
type Schedules map[string]Schedule
