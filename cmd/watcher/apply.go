package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cvhariharan/watcher/internal/config"
	"github.com/spf13/cobra"
	yaml "go.yaml.in/yaml/v4"
)

const (
	kindPolicy      = "policy"
	kindSchedule    = "schedule"
	kindAlertTarget = "alert_target"
	kindAlertRule   = "alert_rule"
	kindYaraSource  = "yara_source"
)

const (
	actionCreated     = "created"
	actionConfigured  = "configured"
	actionWouldCreate = "would create"
	actionWouldUpdate = "would update"
)

func newApplyCmd(flags *rootFlags) *cobra.Command {
	var (
		files     []string
		recursive bool
		dryRun    bool
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply YAML resource definitions to a Watcher server",
		Long: "Apply creates or updates detection content (policies, schedules,\n" +
			"alert rules/targets, YARA signature sources) on a Watcher server from\n" +
			"flat YAML documents. It is idempotent and keyed on each resource's name,\n" +
			"so it is safe to re-run from CI.\n\n" +
			"Authentication uses an API token minted in the UI (Settings → API\n" +
			"Tokens), supplied via --token, WATCHER_TOKEN, or the CLI config file.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(flags, files, recursive, dryRun, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringArrayVarP(&files, "filename", "f", nil, "YAML file, directory, or - for stdin (repeatable)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "recurse into directories")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and report the create/update plan; make no writes")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func runApply(flags *rootFlags, files []string, recursive, dryRun bool, out, errOut io.Writer) error {
	resolved, err := config.ResolveCLIConfig(config.CLIFlags{
		Server:     flags.server,
		Token:      flags.token,
		ConfigPath: flags.config,
		Insecure:   flags.insecure,
	}, errOut)
	if err != nil {
		return err
	}

	cl, err := newAPIClient(resolved.Server, resolved.Token, resolved.Insecure)
	if err != nil {
		return err
	}

	contents, err := gatherInputs(files, recursive)
	if err != nil {
		return err
	}

	var docs []document
	for _, nc := range contents {
		parsed, err := decodeDocuments(bytes.NewReader(nc.data), nc.name)
		if err != nil {
			return err
		}
		docs = append(docs, parsed...)
	}
	if len(docs) == 0 {
		return errors.New("no resources found in the provided input")
	}

	// Validate everything up front so a bad document doesn't leave a
	// half-applied set. The server re-validates per request as the final gate.
	var problems []string
	for _, d := range docs {
		if err := validateDocument(d); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(problems, "\n  "))
	}

	// Stable-sort by dependency order so name references resolve.
	sort.SliceStable(docs, func(i, j int) bool {
		return kindOrder(docs[i].kind) < kindOrder(docs[j].kind)
	})

	rc := newReconciler(cl, dryRun)
	var created, configured, plannedCreate, plannedUpdate, errCount int
	for _, d := range docs {
		label := d.kind + "/" + d.displayName()
		action, err := rc.apply(d)
		if err != nil {
			fmt.Fprintf(out, "%s\terror: %v\n", label, err)
			errCount++
			continue
		}
		fmt.Fprintf(out, "%s\t%s\n", label, action)
		switch action {
		case actionCreated:
			created++
		case actionConfigured:
			configured++
		case actionWouldCreate:
			plannedCreate++
		case actionWouldUpdate:
			plannedUpdate++
		}
	}

	if dryRun {
		fmt.Fprintf(out, "\nPlan: %d to create, %d to update, %d errors\n", plannedCreate, plannedUpdate, errCount)
	} else {
		fmt.Fprintf(out, "\n%d created, %d configured, %d errors\n", created, configured, errCount)
	}

	if errCount > 0 {
		return fmt.Errorf("%d resource(s) failed to apply", errCount)
	}
	return nil
}

// kindOrder returns the dependency-ordering rank for a kind: targets and YARA
// sources first (no deps), then policies/schedules, then alert rules (which
// reference targets).
func kindOrder(kind string) int {
	switch kind {
	case kindAlertTarget:
		return 0
	case kindYaraSource:
		return 1
	case kindPolicy:
		return 2
	case kindSchedule:
		return 3
	case kindAlertRule:
		return 4
	default:
		return 5
	}
}

// ---------------------------------------------------------------------------
// Input gathering
// ---------------------------------------------------------------------------

type namedContent struct {
	name string
	data []byte
}

func gatherInputs(paths []string, recursive bool) ([]namedContent, error) {
	var out []namedContent
	for _, p := range paths {
		if p == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, fmt.Errorf("read stdin: %w", err)
			}
			out = append(out, namedContent{name: "<stdin>", data: data})
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if info.IsDir() {
			files, err := yamlFilesInDir(p, recursive)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				data, err := os.ReadFile(f)
				if err != nil {
					return nil, fmt.Errorf("read %s: %w", f, err)
				}
				out = append(out, namedContent{name: f, data: data})
			}
			continue
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		out = append(out, namedContent{name: p, data: data})
	}
	return out, nil
}

func yamlFilesInDir(dir string, recursive bool) ([]string, error) {
	var files []string
	if recursive {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && isYAMLFile(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if isYAMLFile(path) {
				files = append(files, path)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func isYAMLFile(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".yaml", ".yml":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// YAML schema + decoding
// ---------------------------------------------------------------------------

type document struct {
	kind   string
	name   string
	source string

	policy      *policyDoc
	schedule    *scheduleDoc
	alertTarget *alertTargetDoc
	alertRule   *alertRuleDoc
	yaraSource  *yaraSourceDoc
}

func (d document) displayName() string {
	if d.kind == kindYaraSource && d.yaraSource != nil {
		if strings.TrimSpace(d.yaraSource.Name) != "" {
			return d.yaraSource.Name
		}
		return d.yaraSource.URL
	}
	if strings.TrimSpace(d.name) != "" {
		return d.name
	}
	return "<unnamed>"
}

type policyDoc struct {
	Kind        string   `yaml:"kind"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Query       string   `yaml:"query"`
	Platform    string   `yaml:"platform"`
	Resolution  string   `yaml:"resolution"`
	Enabled     *bool    `yaml:"enabled"`
	Groups      []string `yaml:"groups"`
}

type scheduleDoc struct {
	Kind        string   `yaml:"kind"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Query       string   `yaml:"query"`
	Interval    int      `yaml:"interval"`
	Platform    string   `yaml:"platform"`
	Snapshot    bool     `yaml:"snapshot"`
	Removed     bool     `yaml:"removed"`
	Shard       int      `yaml:"shard"`
	Version     string   `yaml:"version"`
	Groups      []string `yaml:"groups"`
}

type alertTargetDoc struct {
	Kind    string         `yaml:"kind"`
	Name    string         `yaml:"name"`
	Type    string         `yaml:"type"`
	Enabled *bool          `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}

type alertRuleDoc struct {
	Kind               string         `yaml:"kind"`
	Name               string         `yaml:"name"`
	Description        string         `yaml:"description"`
	Source             string         `yaml:"source"`
	Severity           string         `yaml:"severity"`
	Enabled            *bool          `yaml:"enabled"`
	EvaluationInterval int            `yaml:"evaluation_interval"`
	For                int            `yaml:"for"`
	RepeatInterval     int            `yaml:"repeat_interval"`
	Params             map[string]any `yaml:"params"`
	Targets            []string       `yaml:"targets"`
}

type yaraSourceDoc struct {
	Kind    string `yaml:"kind"`
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Group   string `yaml:"group"`
	Enabled *bool  `yaml:"enabled"`
}

func decodeDocuments(r io.Reader, source string) ([]document, error) {
	dec := yaml.NewDecoder(r)
	var docs []document
	for {
		var node yaml.Node
		err := dec.Decode(&node)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}
		if node.Kind == 0 {
			continue // empty document
		}

		var probe struct {
			Kind string `yaml:"kind"`
		}
		if err := node.Decode(&probe); err != nil {
			return nil, fmt.Errorf("%s: read kind: %w", source, err)
		}

		raw, err := yaml.Marshal(&node)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", source, err)
		}

		doc := document{kind: probe.Kind, source: source}
		switch probe.Kind {
		case kindPolicy:
			var v policyDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: policy: %w", source, err)
			}
			doc.name, doc.policy = v.Name, &v
		case kindSchedule:
			var v scheduleDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: schedule: %w", source, err)
			}
			doc.name, doc.schedule = v.Name, &v
		case kindAlertTarget:
			var v alertTargetDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: alert_target: %w", source, err)
			}
			doc.name, doc.alertTarget = v.Name, &v
		case kindAlertRule:
			var v alertRuleDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: alert_rule: %w", source, err)
			}
			doc.name, doc.alertRule = v.Name, &v
		case kindYaraSource:
			var v yaraSourceDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: yara_source: %w", source, err)
			}
			doc.name, doc.yaraSource = v.Name, &v
		case "":
			return nil, fmt.Errorf("%s: document is missing a `kind`", source)
		default:
			return nil, fmt.Errorf("%s: unknown kind %q", source, probe.Kind)
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func strictDecode(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Up-front validation
// ---------------------------------------------------------------------------

var (
	validPlatforms  = map[string]bool{"darwin": true, "linux": true, "posix": true, "windows": true, "any": true, "all": true}
	validSeverities = map[string]bool{"critical": true, "high": true, "medium": true, "low": true, "info": true}
	validTargetType = map[string]bool{"smtp": true, "webhook": true}
)

func validateDocument(d document) error {
	where := d.source + ": " + d.kind + "/" + d.displayName()
	switch d.kind {
	case kindPolicy:
		p := d.policy
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if strings.TrimSpace(p.Query) == "" {
			return fmt.Errorf("%s: query is required", where)
		}
		if p.Platform != "" && !validPlatforms[p.Platform] {
			return fmt.Errorf("%s: invalid platform %q", where, p.Platform)
		}
	case kindSchedule:
		s := d.schedule
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if strings.TrimSpace(s.Query) == "" {
			return fmt.Errorf("%s: query is required", where)
		}
		if s.Interval < 1 || s.Interval > 604800 {
			return fmt.Errorf("%s: interval must be between 1 and 604800 seconds", where)
		}
		if s.Platform != "" && !validPlatforms[s.Platform] {
			return fmt.Errorf("%s: invalid platform %q", where, s.Platform)
		}
		if s.Shard < 0 || s.Shard > 100 {
			return fmt.Errorf("%s: shard must be between 0 and 100", where)
		}
	case kindAlertTarget:
		t := d.alertTarget
		if strings.TrimSpace(t.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if !validTargetType[t.Type] {
			return fmt.Errorf("%s: invalid type %q (want smtp|webhook)", where, t.Type)
		}
	case kindAlertRule:
		r := d.alertRule
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("%s: name is required", where)
		}
		if strings.TrimSpace(r.Source) == "" {
			return fmt.Errorf("%s: source is required", where)
		}
		if !validSeverities[r.Severity] {
			return fmt.Errorf("%s: invalid severity %q", where, r.Severity)
		}
		if r.EvaluationInterval < 60 {
			return fmt.Errorf("%s: evaluation_interval must be >= 60 seconds", where)
		}
	case kindYaraSource:
		y := d.yaraSource
		if strings.TrimSpace(y.URL) == "" {
			return fmt.Errorf("%s: url is required", where)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reconcile
// ---------------------------------------------------------------------------

type reconciler struct {
	cl     *apiClient
	dryRun bool

	groupCache  map[string]string
	targetCache map[string]string
	yaraSources []yaraRemoteSource
	yaraLoaded  bool
}

func newReconciler(cl *apiClient, dryRun bool) *reconciler {
	return &reconciler{
		cl:          cl,
		dryRun:      dryRun,
		groupCache:  map[string]string{},
		targetCache: map[string]string{},
	}
}

func (rc *reconciler) apply(d document) (string, error) {
	switch d.kind {
	case kindPolicy:
		return rc.applyPolicy(d.policy)
	case kindSchedule:
		return rc.applySchedule(d.schedule)
	case kindAlertTarget:
		return rc.applyAlertTarget(d.alertTarget)
	case kindAlertRule:
		return rc.applyAlertRule(d.alertRule)
	case kindYaraSource:
		return rc.applyYaraSource(d.yaraSource)
	default:
		return "", fmt.Errorf("unknown kind %q", d.kind)
	}
}

type policyPayload struct {
	Title       string   `json:"title"`
	Query       string   `json:"query"`
	Description string   `json:"description"`
	Resolution  string   `json:"resolution"`
	Platform    string   `json:"platform,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	GroupIDs    []string `json:"group_ids,omitempty"`
}

func (rc *reconciler) applyPolicy(p *policyDoc) (string, error) {
	groupIDs, err := rc.resolveGroups(p.Groups)
	if err != nil {
		return "", err
	}
	uuid, found, err := rc.cl.lookupUUID("/api/v1/policies/by-name/", p.Name)
	if err != nil {
		return "", err
	}
	body := policyPayload{
		Title:       p.Name,
		Query:       p.Query,
		Description: p.Description,
		Resolution:  p.Resolution,
		Platform:    p.Platform,
		Enabled:     p.Enabled,
		GroupIDs:    groupIDs,
	}
	return rc.write(found, "/api/v1/policies", uuid, body)
}

type schedulePayload struct {
	Title       string   `json:"title"`
	Query       string   `json:"query"`
	Description string   `json:"description"`
	Interval    int      `json:"interval"`
	Platform    string   `json:"platform,omitempty"`
	Snapshot    bool     `json:"snapshot"`
	Removed     bool     `json:"removed"`
	Shard       int      `json:"shard"`
	Version     string   `json:"version,omitempty"`
	GroupIDs    []string `json:"group_ids,omitempty"`
}

func (rc *reconciler) applySchedule(s *scheduleDoc) (string, error) {
	groupIDs, err := rc.resolveGroups(s.Groups)
	if err != nil {
		return "", err
	}
	uuid, found, err := rc.cl.lookupUUID("/api/v1/schedules/by-name/", s.Name)
	if err != nil {
		return "", err
	}
	body := schedulePayload{
		Title:       s.Name,
		Query:       s.Query,
		Description: s.Description,
		Interval:    s.Interval,
		Platform:    s.Platform,
		Snapshot:    s.Snapshot,
		Removed:     s.Removed,
		Shard:       s.Shard,
		Version:     s.Version,
		GroupIDs:    groupIDs,
	}
	return rc.write(found, "/api/v1/schedules", uuid, body)
}

type alertTargetCreatePayload struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

type alertTargetUpdatePayload struct {
	Name    string          `json:"name"`
	Config  json.RawMessage `json:"config,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

func (rc *reconciler) applyAlertTarget(t *alertTargetDoc) (string, error) {
	cfg, err := marshalRaw(t.Config)
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	uuid, found, err := rc.cl.lookupUUID("/api/v1/alert-targets/by-name/", t.Name)
	if err != nil {
		return "", err
	}
	if rc.dryRun {
		return planAction(found), nil
	}
	if found {
		if err := rc.cl.put("/api/v1/alert-targets/"+uuid, alertTargetUpdatePayload{Name: t.Name, Config: cfg, Enabled: t.Enabled}); err != nil {
			return "", err
		}
		return actionConfigured, nil
	}
	if err := rc.cl.post("/api/v1/alert-targets", alertTargetCreatePayload{Name: t.Name, Type: t.Type, Config: cfg, Enabled: t.Enabled}); err != nil {
		return "", err
	}
	return actionCreated, nil
}

type alertRulePayload struct {
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Source             string          `json:"source"`
	Params             json.RawMessage `json:"params,omitempty"`
	Severity           string          `json:"severity"`
	Enabled            *bool           `json:"enabled,omitempty"`
	EvaluationInterval int             `json:"evaluation_interval_seconds"`
	For                int             `json:"for_seconds"`
	RepeatInterval     int             `json:"repeat_interval_seconds"`
	TargetIDs          []string        `json:"target_ids,omitempty"`
}

func (rc *reconciler) applyAlertRule(r *alertRuleDoc) (string, error) {
	targetIDs, err := rc.resolveTargets(r.Targets)
	if err != nil {
		return "", err
	}
	params, err := marshalRaw(r.Params)
	if err != nil {
		return "", fmt.Errorf("encode params: %w", err)
	}
	uuid, found, err := rc.cl.lookupUUID("/api/v1/alert-rules/by-name/", r.Name)
	if err != nil {
		return "", err
	}
	body := alertRulePayload{
		Name:               r.Name,
		Description:        r.Description,
		Source:             r.Source,
		Params:             params,
		Severity:           r.Severity,
		Enabled:            r.Enabled,
		EvaluationInterval: r.EvaluationInterval,
		For:                r.For,
		RepeatInterval:     r.RepeatInterval,
		TargetIDs:          targetIDs,
	}
	return rc.write(found, "/api/v1/alert-rules", uuid, body)
}

type yaraSourcePayload struct {
	GroupID string `json:"group_id,omitempty"`
	URL     string `json:"url"`
	Label   string `json:"label"`
	Enabled *bool  `json:"enabled,omitempty"`
}

func (rc *reconciler) applyYaraSource(y *yaraSourceDoc) (string, error) {
	groupID := ""
	if strings.TrimSpace(y.Group) != "" {
		id, err := rc.resolveGroup(y.Group)
		if err != nil {
			return "", err
		}
		groupID = id
	}

	sources, err := rc.yaraList()
	if err != nil {
		return "", err
	}
	uuid, found := matchYaraSource(sources, y.URL, groupID)

	body := yaraSourcePayload{GroupID: groupID, URL: y.URL, Label: y.Name, Enabled: y.Enabled}
	return rc.write(found, "/api/v1/yara/signature-sources", uuid, body)
}

func matchYaraSource(sources []yaraRemoteSource, url, groupID string) (string, bool) {
	for _, s := range sources {
		if s.URL == url && s.GroupID == groupID {
			return s.UUID, true
		}
	}
	return "", false
}

// write performs the create/update for resources whose POST and PUT bodies are
// identical. Returns the reported action.
func (rc *reconciler) write(exists bool, basePath, uuid string, body any) (string, error) {
	if rc.dryRun {
		return planAction(exists), nil
	}
	if exists {
		if err := rc.cl.put(basePath+"/"+uuid, body); err != nil {
			return "", err
		}
		return actionConfigured, nil
	}
	if err := rc.cl.post(basePath, body); err != nil {
		return "", err
	}
	return actionCreated, nil
}

func planAction(exists bool) string {
	if exists {
		return actionWouldUpdate
	}
	return actionWouldCreate
}

func (rc *reconciler) resolveGroups(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		id, err := rc.resolveGroup(n)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func (rc *reconciler) resolveGroup(name string) (string, error) {
	if id, ok := rc.groupCache[name]; ok {
		return id, nil
	}
	id, found, err := rc.cl.lookupUUID("/api/v1/groups/by-name/", name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("unknown machine group %q", name)
	}
	rc.groupCache[name] = id
	return id, nil
}

func (rc *reconciler) resolveTargets(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if id, ok := rc.targetCache[n]; ok {
			out = append(out, id)
			continue
		}
		id, found, err := rc.cl.lookupUUID("/api/v1/alert-targets/by-name/", n)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("unknown alert target %q", n)
		}
		rc.targetCache[n] = id
		out = append(out, id)
	}
	return out, nil
}

func (rc *reconciler) yaraList() ([]yaraRemoteSource, error) {
	if !rc.yaraLoaded {
		sources, err := rc.cl.listYaraSources()
		if err != nil {
			return nil, err
		}
		rc.yaraSources = sources
		rc.yaraLoaded = true
	}
	return rc.yaraSources, nil
}

// marshalRaw encodes a YAML-decoded map as compact JSON for pass-through to the
// API's json.RawMessage config/params fields. An empty map yields nil.
func marshalRaw(m map[string]any) (json.RawMessage, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
