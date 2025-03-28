package models

type Node struct {
	NodeKey        string        `json:"node_key"`
	HostIdentifier string        `json:"host_identifier"`
	OSVersion      OSVersionInfo `json:"os_version"`
	OSQuery        OsqueryInfo   `json:"osquery_info"`
	System         SystemInfo    `json:"system_info"`
	Platform       PlatformInfo  `json:"platform_info"`
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
	OsqueryUUID   string `json:"uuid"`
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
	UUID        string `json:"uuid"`
	Query       string `json:"query"`
	Description string `json:"description"`
}

type Schedule struct {
	Query
	UUID     string `json:"uuid"`
	Interval int    `json:"interval"`
	Removed  bool   `json:"removed"`
	Snapshot bool   `json:"snapshot"`
	Platform string `json:"platform"`
	Version  string `json:"version"`
	Shard    int    `json:"shard"`
	Denylist bool   `json:"denylist"`
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
