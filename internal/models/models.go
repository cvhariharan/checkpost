package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AlertTarget is a delivery destination. Config is opaque to the engine.
type AlertTarget struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type AlertRule struct {
	UUID               string          `json:"uuid"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Source             string          `json:"source"`
	Params             json.RawMessage `json:"params"`
	Severity           string          `json:"severity"`
	Enabled            bool            `json:"enabled"`
	EvaluationInterval int             `json:"evaluation_interval_seconds"`
	For                int             `json:"for_seconds"`
	RepeatInterval     int             `json:"repeat_interval_seconds"`
	TargetIDs          []string        `json:"target_ids"`
	LastEvaluatedAt    *time.Time      `json:"last_evaluated_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// AlertSourceInfo describes a registered source type and its params JSON Schema.
type AlertSourceInfo struct {
	Type   string `json:"type"`
	Schema any    `json:"schema"`
}

type CreateAlertTarget struct {
	Name    string
	Type    string
	Config  json.RawMessage
	Enabled bool
}

type UpdateAlertTarget struct {
	UUID    string
	Name    string
	Config  json.RawMessage
	Enabled bool
}

type CreateAlertRule struct {
	Name               string
	Description        string
	Source             string
	Params             json.RawMessage
	Severity           string
	Enabled            bool
	EvaluationInterval int
	For                int
	RepeatInterval     int
	TargetIDs          []string
}

type UpdateAlertRule struct {
	UUID string
	CreateAlertRule
}

type Node struct {
	ID                int64           `json:"-"`
	ResourceID        string          `json:"id"`
	UUID              string          `json:"uuid"`
	NodeKey           string          `json:"node_key"`
	HostIdentifier    string          `json:"host_identifier"`
	Hostname          string          `json:"hostname"`
	DisplayName       string          `json:"display_name"`
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
	ComplianceScore   *int            `json:"compliance_score,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	EnrollmentInput   HostDetailsInfo `json:"-"`
}

type NodeEnrollment struct {
	HostIdentifier string
	HostDetails    HostDetailsInfo

	// OwnerUserUUID is the checkpost user embedded in an owner-bound enrollment
	// secret. uuid.Nil for anonymous/legacy enrollments.
	OwnerUserUUID uuid.UUID
}

type NodeCredentials struct {
	NodeKey string `json:"node_key"`
}

type UpdateNode struct {
	UUID        string
	DisplayName string
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
	Checkpost     string `json:"checkpost"`
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
	Columns         []string            `json:"columns"`
	Rows            []ScheduleResultRow `json:"rows"`
	Total           int                 `json:"total"`
	Page            int                 `json:"page"`
	CountPerPage    int                 `json:"count_per_page"`
	PageCount       int                 `json:"page_count"`
	ExportSupported bool                `json:"export_supported"`
	// BrowsingDisabled is set when no reader backend is enabled
	BrowsingDisabled bool `json:"browsing_disabled,omitempty"`
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
	Severity           string     `json:"severity"`
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
	Severity    string
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
	Severity    string
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
	Query string
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
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	RowCount  int       `json:"row_count"`
	Error     string    `json:"error,omitempty"`
}

// AdHocQueryResults is one page of an ad-hoc/run query's result rows.
type AdHocQueryResults struct {
	Columns          []string            `json:"columns"`
	Rows             []map[string]string `json:"rows"`
	Total            int                 `json:"total"`
	Page             int                 `json:"page"`
	CountPerPage     int                 `json:"count_per_page"`
	PageCount        int                 `json:"page_count"`
	ExportSupported  bool                `json:"export_supported"`
	Pending          bool                `json:"pending,omitempty"`
	BrowsingDisabled bool                `json:"browsing_disabled,omitempty"`
	Error            string              `json:"error,omitempty"`
}

// QueryTargets is the set of selectors that resolve to the hosts a query run
// targets. The three selectors are unioned (a host matching any is included).
type QueryTargets struct {
	HostIDs   []string `json:"host_ids"`
	GroupIDs  []string `json:"group_ids"`
	Platforms []string `json:"platforms"`
}

// QueryRunRequest is the input for creating a multi-host query run.
type QueryRunRequest struct {
	Query         string
	Targets       QueryTargets
	CreatedByUUID string
}

// QueryRunHost is a single host's execution within a query run.
type QueryRunHost struct {
	QueryID   string    `json:"query_id"`
	NodeUUID  string    `json:"node_uuid"`
	Hostname  string    `json:"hostname"`
	Platform  string    `json:"platform"`
	Status    string    `json:"status"`
	RowCount  int       `json:"row_count"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// QueryRun groups one SQL submission fanned out to many hosts. Hosts is
// populated only on the single-run (detail) view.
type QueryRun struct {
	ID            string         `json:"id"`
	Query         string         `json:"query"`
	Targets       QueryTargets   `json:"targets"`
	HostCount     int            `json:"host_count"`
	PendingCount  int            `json:"pending_count"`
	CompleteCount int            `json:"complete_count"`
	ErrorCount    int            `json:"error_count"`
	CreatedAt     time.Time      `json:"created_at"`
	Hosts         []QueryRunHost `json:"hosts,omitempty"`
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
	Paths    []string
	GroupID  string
	RuleURLs []string
}

// AUTHENTICATION & AUTHORIZATION ------------------------------------------

const (
	LoginTypeStandard = "standard"
	LoginTypeOIDC     = "oidc"

	UserGroupSourceManual = "manual"
	UserGroupSourceOIDC   = "oidc"
)

// User is a human account. SSO users have LoginType "oidc" and no password.
type User struct {
	ID          int64      `json:"-"`
	UUID        string     `json:"uuid"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	LoginType   string     `json:"login_type"`
	Disabled    bool       `json:"disabled"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// SessionUser is the minimal user identity stored in the session cookie store.
type SessionUser struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	LoginType string `json:"login_type"`
}

// APIToken is the API-facing view of a token row (no secret, no hash).
type APIToken struct {
	UUID       string     `json:"uuid"`
	Name       string     `json:"name"`
	Source     string     `json:"source"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IssuedAPIToken adds the plaintext secret, returned only once at creation.
type IssuedAPIToken struct {
	APIToken
	Secret string `json:"secret"`
}

type CreateUser struct {
	Name  string
	Email string
}

type UpdateUser struct {
	UUID     string
	Name     string
	Email    string
	Disabled bool
}

// OIDCClaims are the ID-token claims checkpost reads on SSO login.
type OIDCClaims struct {
	Email  string
	Name   string
	Groups []string
}

type UserGroup struct {
	ID             int64     `json:"-"`
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	OIDCClaimValue string    `json:"oidc_claim_value"`
	MemberCount    int       `json:"member_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateUserGroup struct {
	Name           string
	Description    string
	OIDCClaimValue string
}

type UpdateUserGroup struct {
	UUID           string
	Name           string
	Description    string
	OIDCClaimValue string
}

type UserGroupMember struct {
	UserUUID  string `json:"user_uuid"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	LoginType string `json:"login_type"`
	Disabled  bool   `json:"disabled"`
	Source    string `json:"source"`
}

// RoleBinding ties a subject (user or user group) to a built-in role, optionally
// scoped to a single machine group (nil ScopeGroup = global).
type RoleBinding struct {
	UUID           string    `json:"uuid"`
	Role           string    `json:"role"`
	ScopeGroupUUID string    `json:"scope_group_uuid,omitempty"`
	ScopeGroupName string    `json:"scope_group_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// RoleDefinition is a built-in role and its permission matrix (resource -> actions).
type RoleDefinition struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Permissions map[string][]string `json:"permissions"`
}

// PermissionCatalogEntry describes one resource and the actions it supports.
type PermissionCatalogEntry struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
	Scopable bool     `json:"scopable"`
}

type Catalog struct {
	Resources []PermissionCatalogEntry `json:"resources"`
}

// EffectivePermissions is returned to the frontend for UI gating.
type EffectivePermissions struct {
	User        SessionUser         `json:"user"`
	Roles       []string            `json:"roles"`
	Permissions map[string][]string `json:"permissions"`
}

type YaraScan struct {
	ID             string     `json:"id"`
	UUID           string     `json:"uuid"`
	GroupID        string     `json:"group_id,omitempty"`
	GroupName      string     `json:"group_name,omitempty"`
	Paths          []string   `json:"paths"`
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

// LookupResult is the minimal by-name lookup payload used by `checkpost apply` to
// decide create vs update
type LookupResult struct {
	UUID string
	Name string
}

// DashboardOverview is the full aggregate payload for the machine overview dashboard.
type DashboardOverview struct {
	GeneratedAt               time.Time                  `json:"generated_at"`
	HeartbeatThresholdSeconds int                        `json:"heartbeat_threshold_seconds"`
	Machines                  DashboardMachines          `json:"machines"`
	Compliance                DashboardCompliance        `json:"compliance"`
	Security                  DashboardSecurity          `json:"security"`
	RecentlyEnrolled          []DashboardEnrolledMachine `json:"recently_enrolled"`
}

type DashboardEnrolledMachine struct {
	UUID        string    `json:"uuid"`
	Hostname    string    `json:"hostname"`
	DisplayName string    `json:"display_name"`
	EnrolledAt  time.Time `json:"enrolled_at"`
}

type DashboardMachines struct {
	Total         int                      `json:"total"`
	Online        int                      `json:"online"`
	Offline       int                      `json:"offline"`
	NeverReported int                      `json:"never_reported"`
	ByPlatform    []DashboardPlatformCount `json:"by_platform"`
}

type DashboardPlatformCount struct {
	Platform string `json:"platform"`
	Total    int    `json:"total"`
	Online   int    `json:"online"`
}

type DashboardCompliance struct {
	Score              *int                      `json:"score"`
	PolicyRows         DashboardPolicyRows       `json:"policy_rows"`
	TopFailingPolicies []DashboardFailingPolicy  `json:"top_failing_policies"`
	LeastCompliant     []DashboardComplianceNode `json:"least_compliant"`
	MostCompliant      []DashboardComplianceNode `json:"most_compliant"`
}

type DashboardPolicyRows struct {
	Passing int `json:"passing"`
	Failing int `json:"failing"`
	Unknown int `json:"unknown"`
}

type DashboardFailingPolicy struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	FailingCount int    `json:"failing_count"`
	Platform     string `json:"platform"`
}

type DashboardComplianceNode struct {
	UUID        string `json:"uuid"`
	Hostname    string `json:"hostname"`
	DisplayName string `json:"display_name"`
	Score       int    `json:"score"`
	Passing     int    `json:"passing"`
	Failing     int    `json:"failing"`
	Total       int    `json:"total"`
}

type DashboardSecurity struct {
	FiringAlerts      DashboardFiringAlerts  `json:"firing_alerts"`
	FiringAlertList   []DashboardFiringAlert `json:"firing_alert_list"`
	RecentYaraMatches []DashboardYaraMatch   `json:"recent_yara_matches"`
}

type DashboardFiringAlerts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
	Total    int `json:"total"`
}

type DashboardFiringAlert struct {
	UUID       string    `json:"uuid"`
	Name       string    `json:"name"`
	Severity   string    `json:"severity"`
	Count      int       `json:"count"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type DashboardYaraMatch struct {
	ScanUUID    string    `json:"scan_uuid"`
	MachineUUID string    `json:"machine_uuid"`
	Hostname    string    `json:"hostname"`
	Path        string    `json:"path"`
	Rules       string    `json:"rules"`
	MatchedAt   time.Time `json:"matched_at"`
}
