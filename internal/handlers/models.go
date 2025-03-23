package handlers

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
