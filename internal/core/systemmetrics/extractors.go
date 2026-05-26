package systemmetrics

import (
	"sort"
	"strconv"
	"strings"

	"github.com/cvhariharan/watcher/internal/results"
	"github.com/invopop/jsonschema"
)

// reflector emits inline definitions (no $ref / $defs indirection) which
// makes the rendered schema easier to walk on the frontend.
var reflector = &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}

func schemaOf(v any, title string) *jsonschema.Schema {
	s := reflector.Reflect(v)
	s.Title = title
	return s
}

// --- disk ---------------------------------------------------------------

type DiskValue struct {
	Mounts []DiskMount `json:"mounts" jsonschema:"title=Mounts" jsonschema_extras:"x-display=table"`
}

type DiskMount struct {
	Path    string  `json:"path"           jsonschema:"title=Path"`
	Type    string  `json:"type,omitempty" jsonschema:"title=Filesystem"`
	FreeGB  float64 `json:"free_gb"        jsonschema:"title=Free"   jsonschema_extras:"x-unit=gigabytes"`
	TotalGB float64 `json:"total_gb"       jsonschema:"title=Total"  jsonschema_extras:"x-unit=gigabytes"`
	FreePct float64 `json:"free_pct"       jsonschema:"title=Free %" jsonschema_extras:"x-progress=inverse,x-unit=percent"`
}

type DiskExtractor struct{}

func (DiskExtractor) Kind() Kind                  { return KindDisk }
func (DiskExtractor) Schema() *jsonschema.Schema  { return schemaOf(&DiskValue{}, "Disk Usage") }

func (DiskExtractor) Extract(rows []results.Row) (any, bool) {
	mounts := make([]DiskMount, 0, len(rows))
	for _, row := range rows {
		path := strings.TrimSpace(row.Columns["path"])
		if path == "" {
			continue
		}
		mounts = append(mounts, DiskMount{
			Path:    path,
			Type:    row.Columns["type"],
			FreeGB:  parseFloat(row.Columns["free_gb"]),
			TotalGB: parseFloat(row.Columns["total_gb"]),
			FreePct: parseFloat(row.Columns["free_perc"]),
		})
	}
	if len(mounts) == 0 {
		return nil, false
	}
	sort.Slice(mounts, func(i, j int) bool { return mounts[i].Path < mounts[j].Path })
	return DiskValue{Mounts: mounts}, true
}

// --- network ------------------------------------------------------------

type NetworkValue struct {
	Interfaces []NetworkInterface `json:"interfaces" jsonschema:"title=Interfaces" jsonschema_extras:"x-display=table"`
}

type NetworkInterface struct {
	Name      string   `json:"name"          jsonschema:"title=Interface"`
	MAC       string   `json:"mac,omitempty" jsonschema:"title=MAC"`
	Addresses []string `json:"addresses"     jsonschema:"title=Addresses" jsonschema_extras:"x-display=pills"`
}

type NetworkExtractor struct{}

func (NetworkExtractor) Kind() Kind                 { return KindNetwork }
func (NetworkExtractor) Schema() *jsonschema.Schema { return schemaOf(&NetworkValue{}, "Network Interfaces") }

func (NetworkExtractor) Extract(rows []results.Row) (any, bool) {
	type acc struct {
		mac  string
		seen map[string]struct{}
		list []string
	}
	byName := make(map[string]*acc)
	for _, row := range rows {
		name := strings.TrimSpace(row.Columns["name"])
		if name == "" {
			continue
		}
		a := byName[name]
		if a == nil {
			a = &acc{seen: make(map[string]struct{})}
			byName[name] = a
		}
		if mac := strings.TrimSpace(row.Columns["mac"]); mac != "" && a.mac == "" {
			a.mac = mac
		}
		addr := strings.TrimSpace(row.Columns["address"])
		if addr == "" {
			continue
		}
		if _, ok := a.seen[addr]; ok {
			continue
		}
		a.seen[addr] = struct{}{}
		a.list = append(a.list, addr)
	}
	if len(byName) == 0 {
		return nil, false
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	ifaces := make([]NetworkInterface, 0, len(names))
	for _, n := range names {
		a := byName[n]
		ifaces = append(ifaces, NetworkInterface{Name: n, MAC: a.mac, Addresses: a.list})
	}
	return NetworkValue{Interfaces: ifaces}, true
}

// --- memory -------------------------------------------------------------

// AvailableBytes is omitempty so the Windows SQL — which doesn't expose
// available memory — can leave it unset and the renderer treats it as
// "not reported" instead of falsely showing zero free memory.
type MemoryValue struct {
	TotalBytes     int64 `json:"total_bytes"               jsonschema:"title=Total"     jsonschema_extras:"x-unit=bytes,x-primary=true"`
	AvailableBytes int64 `json:"available_bytes,omitempty" jsonschema:"title=Available" jsonschema_extras:"x-unit=bytes"`
}

type MemoryExtractor struct{}

func (MemoryExtractor) Kind() Kind                 { return KindMemory }
func (MemoryExtractor) Schema() *jsonschema.Schema { return schemaOf(&MemoryValue{}, "Memory") }

func (MemoryExtractor) Extract(rows []results.Row) (any, bool) {
	for _, row := range rows {
		total := parseInt(row.Columns["total_bytes"])
		avail := parseInt(row.Columns["available_bytes"])
		if total == 0 && avail == 0 {
			continue
		}
		return MemoryValue{TotalBytes: total, AvailableBytes: avail}, true
	}
	return nil, false
}

// --- cpu ----------------------------------------------------------------

type CPUValue struct {
	Model         string `json:"model"          jsonschema:"title=Model" jsonschema_extras:"x-primary=true"`
	PhysicalCores int    `json:"physical_cores" jsonschema:"title=Physical Cores"`
	LogicalCores  int    `json:"logical_cores"  jsonschema:"title=Logical Cores"`
}

type CPUExtractor struct{}

func (CPUExtractor) Kind() Kind                 { return KindCPU }
func (CPUExtractor) Schema() *jsonschema.Schema { return schemaOf(&CPUValue{}, "CPU") }

func (CPUExtractor) Extract(rows []results.Row) (any, bool) {
	for _, row := range rows {
		model := strings.TrimSpace(row.Columns["model"])
		physical := int(parseInt(row.Columns["physical_cores"]))
		logical := int(parseInt(row.Columns["logical_cores"]))
		if model == "" && physical == 0 && logical == 0 {
			continue
		}
		return CPUValue{Model: model, PhysicalCores: physical, LogicalCores: logical}, true
	}
	return nil, false
}

// --- os_info ------------------------------------------------------------

type OSInfoValue struct {
	Name     string `json:"name"               jsonschema:"title=Name"    jsonschema_extras:"x-primary=true"`
	Version  string `json:"version"            jsonschema:"title=Version"`
	Build    string `json:"build,omitempty"    jsonschema:"title=Build"`
	Platform string `json:"platform,omitempty" jsonschema:"title=Platform"`
}

type OSInfoExtractor struct{}

func (OSInfoExtractor) Kind() Kind                 { return KindOSInfo }
func (OSInfoExtractor) Schema() *jsonschema.Schema { return schemaOf(&OSInfoValue{}, "Operating System") }

func (OSInfoExtractor) Extract(rows []results.Row) (any, bool) {
	for _, row := range rows {
		name := strings.TrimSpace(row.Columns["name"])
		version := strings.TrimSpace(row.Columns["version"])
		if name == "" && version == "" {
			continue
		}
		return OSInfoValue{
			Name:     name,
			Version:  version,
			Build:    row.Columns["build"],
			Platform: row.Columns["platform"],
		}, true
	}
	return nil, false
}

// --- uptime -------------------------------------------------------------

type UptimeValue struct {
	Seconds int64 `json:"seconds" jsonschema:"title=Uptime" jsonschema_extras:"x-unit=duration-seconds,x-primary=true"`
}

type UptimeExtractor struct{}

func (UptimeExtractor) Kind() Kind                 { return KindUptime }
func (UptimeExtractor) Schema() *jsonschema.Schema { return schemaOf(&UptimeValue{}, "Uptime") }

func (UptimeExtractor) Extract(rows []results.Row) (any, bool) {
	for _, row := range rows {
		secs := parseInt(row.Columns["seconds"])
		if secs <= 0 {
			continue
		}
		return UptimeValue{Seconds: secs}, true
	}
	return nil, false
}

// --- helpers ------------------------------------------------------------

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}
