package models

import "time"

type Node struct {
	ID                int64           `json:"-"`
	ResourceID        string          `json:"id"`
	UUID              string          `json:"uuid"`
	NodeKey           string          `json:"node_key"`
	HostIdentifier    string          `json:"host_identifier"`
	Hostname          string          `json:"hostname"`
	Platform          string          `json:"platform"`
	OSName            string          `json:"os_name"`
	OSVersion         string          `json:"os_version"`
	OSQueryVersion    string          `json:"osquery_version"`
	HardwareSerial    string          `json:"hardware_serial"`
	EnrolledAt        time.Time       `json:"enrolled_at"`
	LastSeenAt        *time.Time      `json:"last_seen_at,omitempty"`
	LastPolicyCheckAt *time.Time      `json:"last_policy_check_at,omitempty"`
	Groups            []Group         `json:"groups,omitempty"`
	Inventory         *NodeInventory  `json:"inventory,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	EnrollmentInput   HostDetailsInfo `json:"-"`
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

type Schedule struct {
	ID                int64     `json:"-"`
	UUID              string    `json:"uuid"`
	Name              string    `json:"name"`
	Title             string    `json:"title"`
	SQL               string    `json:"sql"`
	Description       string    `json:"description"`
	VersionedName     string    `json:"versioned_name"`
	SQLVersion        int       `json:"sql_version"`
	IntervalSeconds   int       `json:"interval_seconds"`
	Interval          int       `json:"interval"`
	Removed           bool      `json:"removed"`
	Snapshot          bool      `json:"snapshot"`
	Platform          string    `json:"platform"`
	Version           string    `json:"version"`
	Shard             int       `json:"shard"`
	Enabled           bool      `json:"enabled"`
	IsSystem          bool      `json:"is_system"`
	Denylist          bool      `json:"denylist"`
	Groups            []Group   `json:"groups,omitempty"`
	TargetAllMachines bool      `json:"target_all_machines"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CreateSchedule struct {
	Name            string
	SQL             string
	Description     string
	IntervalSeconds int
	Removed         bool
	Snapshot        bool
	Platform        string
	Version         string
	Shard           int
	Denylist        bool
	Enabled         bool
	IsSystem        bool
	GroupIDs        []string
}

type UpdateSchedule struct {
	UUID            string
	Name            string
	SQL             string
	Description     string
	IntervalSeconds int
	Removed         bool
	Snapshot        bool
	Platform        string
	Version         string
	Shard           int
	Denylist        bool
	Enabled         bool
	RetentionDays   int
	GroupIDs        []string
}

type ScheduleResultRow struct {
	NodeUUID string            `json:"node_uuid"`
	Hostname string            `json:"hostname"`
	Columns  map[string]string `json:"columns"`
	LastSeen time.Time         `json:"last_seen"`
}

type ScheduleResults struct {
	Columns      []string            `json:"columns"`
	Rows         []ScheduleResultRow `json:"rows"`
	Total        int                 `json:"total"`
	Page         int                 `json:"page"`
	CountPerPage int                 `json:"count_per_page"`
	PageCount    int                 `json:"page_count"`
}

type ScheduleResultsRequest struct {
	ScheduleUUID string
	Page         int
	Count        int
	Query        string
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
	Groups             []Group    `json:"groups,omitempty"`
	TargetAllMachines  bool       `json:"target_all_machines"`
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
	GroupIDs    []string
}

type UpdatePolicy struct {
	UUID        string
	Name        string
	Query       string
	Description string
	Resolution  string
	Platform    string
	Enabled     bool
	GroupIDs    []string
}

type Group struct {
	ID           int64     `json:"-"`
	ResourceID   string    `json:"id"`
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	MachineCount int       `json:"machine_count"`
	PolicyCount  int       `json:"policy_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateGroup struct {
	Name        string
	Description string
}

type UpdateGroup struct {
	UUID        string
	Name        string
	Description string
}

type GroupMachinesRequest struct {
	GroupUUID string
	Page      int
	Count     int
}

type DeviceOwner struct {
	ID           int64     `json:"-"`
	ResourceID   string    `json:"id"`
	UUID         string    `json:"uuid"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	ExternalID   string    `json:"external_id"`
	Department   string    `json:"department"`
	Title        string    `json:"title"`
	Phone        string    `json:"phone"`
	Notes        string    `json:"notes"`
	MachineCount int       `json:"machine_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateDeviceOwner struct {
	DisplayName string
	Email       string
	ExternalID  string
	Department  string
	Title       string
	Phone       string
	Notes       string
}

type UpdateDeviceOwner struct {
	UUID        string
	DisplayName string
	Email       string
	ExternalID  string
	Department  string
	Title       string
	Phone       string
	Notes       string
}

type DeviceOwnerListRequest struct {
	Page  int
	Count int
	Query string
}

type NodeListRequest struct {
	Page     int
	Count    int
	Query    string
	Platform string
	OwnerID  string
	Assigned string
}

type OwnerMachinesRequest struct {
	OwnerUUID string
	Page      int
	Count     int
}

type NodeInventory struct {
	InternalTrackingID string       `json:"internal_tracking_id"`
	Notes              string       `json:"notes"`
	Owner              *DeviceOwner `json:"owner,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

type UpdateNodeInventory struct {
	NodeUUID           string
	OwnerUUID          string
	InternalTrackingID string
	Notes              string
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

type YaraSignatureSource struct {
	ID        string    `json:"id"`
	UUID      string    `json:"uuid"`
	GroupID   string    `json:"group_id,omitempty"`
	GroupName string    `json:"group_name,omitempty"`
	URL       string    `json:"url"`
	Label     string    `json:"label"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type YaraSignatureSourceRequest struct {
	UUID    string
	GroupID string
	URL     string
	Label   string
	Enabled bool
}

type YaraScanRequest struct {
	Path     string
	GroupID  string
	RuleURLs []string
}

type YaraScan struct {
	ID             string     `json:"id"`
	UUID           string     `json:"uuid"`
	GroupID        string     `json:"group_id,omitempty"`
	GroupName      string     `json:"group_name,omitempty"`
	Path           string     `json:"path"`
	Status         string     `json:"status"`
	TargetCount    int        `json:"target_count"`
	CompletedCount int        `json:"completed_count"`
	MatchCount     int        `json:"match_count"`
	Error          string     `json:"error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type YaraScanMatch struct {
	MachineUUID string    `json:"machine_uuid"`
	Hostname    string    `json:"hostname"`
	Path        string    `json:"path"`
	Matches     string    `json:"matches"`
	Count       int       `json:"count"`
	CreatedAt   time.Time `json:"created_at"`
}

type YaraScanTarget struct {
	MachineUUID  string     `json:"machine_uuid"`
	Hostname     string     `json:"hostname"`
	Status       string     `json:"status"`
	DispatchedAt *time.Time `json:"dispatched_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type YaraSignatureSourcesResponse struct {
	Sources    []YaraSignatureSource `json:"sources"`
	TotalCount int                   `json:"total_count"`
	PageCount  int                   `json:"page_count"`
}

type YaraScansResponse struct {
	Scans      []YaraScan `json:"scans"`
	TotalCount int        `json:"total_count"`
	PageCount  int        `json:"page_count"`
}

type YaraScanMatchesResponse struct {
	Matches    []YaraScanMatch `json:"matches"`
	TotalCount int             `json:"total_count"`
	PageCount  int             `json:"page_count"`
}

type YaraScanTargetsResponse struct {
	Targets    []YaraScanTarget `json:"targets"`
	TotalCount int              `json:"total_count"`
	PageCount  int              `json:"page_count"`
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

type Schedules map[string]Schedule

// NodeMetric is one device-metric snapshot for a host, keyed by kind.
type NodeMetric struct {
	Kind        string      `json:"kind"`
	Value       interface{} `json:"value"`
	CollectedAt time.Time   `json:"collected_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
