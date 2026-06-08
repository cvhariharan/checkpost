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
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/cvhariharan/checkpost/internal/config"
	"github.com/go-playground/validator/v10"
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
		statePath string
		prune     bool
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply YAML resource definitions to a Checkpost server",
		Long: `Apply creates or updates content (policies, schedules, alert
rules/targets, YARA signature sources) on a Checkpost server from flat YAML
documents. It is idempotent and safe to run from CI.

Authentication uses an API token (Settings → API Tokens), supplied via --token,
CHECKPOST_TOKEN, or the CLI config file.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(flags, files, recursive, dryRun, statePath, prune, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringArrayVarP(&files, "filename", "f", nil, "YAML file, directory, or - for stdin (repeatable)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "recurse into directories")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and report the plan; make no writes")
	cmd.Flags().StringVar(&statePath, "state", defaultStatePath, "path to the committed handle->UUID state file")
	cmd.Flags().BoolVar(&prune, "prune", false,
		"delete state-tracked resources absent from the applied YAML; compares against ONLY the documents in this run, so apply the complete set")
	_ = cmd.MarkFlagRequired("filename")

	return cmd
}

func runApply(flags *rootFlags, files []string, recursive, dryRun bool, statePath string, prune bool, out, errOut io.Writer) error {
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
	v := newDocValidator()
	var problems []string
	seen := map[string]bool{} // kind|id, guards against duplicate handles
	for _, d := range docs {
		if err := validateDocument(v, d); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		key := d.kind + "|" + d.id
		if seen[key] {
			problems = append(problems, fmt.Sprintf("%s: duplicate id %q for kind %s", d.source, d.id, d.kind))
		}
		seen[key] = true
	}
	if len(problems) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(problems, "\n  "))
	}

	state, err := loadState(statePath)
	if err != nil {
		return err
	}

	// Stable-sort by dependency order so references resolve (targets before
	// the rules that reference them, etc.).
	sort.SliceStable(docs, func(i, j int) bool {
		return kindOrder(docs[i].kind) < kindOrder(docs[j].kind)
	})

	rc := newReconciler(cl, state, dryRun)
	var created, configured, plannedCreate, plannedUpdate, errCount int
	for _, d := range docs {
		label := d.kind + "/" + d.identifier()
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

	// Prune resources tracked in state but absent from the applied docs.
	deleted, plannedDelete, skipped := pruneOrphans(rc, docs, prune, dryRun, out, &errCount)

	// Persist the updated identity map (dry-run stays side-effect free).
	if !dryRun {
		if err := state.save(statePath); err != nil {
			return err
		}
	}

	if dryRun {
		fmt.Fprintf(out, "\nPlan: %d to create, %d to update, %d to delete, %d errors\n", plannedCreate, plannedUpdate, plannedDelete, errCount)
	} else {
		fmt.Fprintf(out, "\n%d created, %d configured, %d deleted, %d errors\n", created, configured, deleted, errCount)
		if skipped > 0 {
			fmt.Fprintf(out, "%d orphaned resource(s) left in place; re-run with --prune to delete\n", skipped)
		}
	}

	if errCount > 0 {
		return fmt.Errorf("%d resource(s) failed to apply", errCount)
	}
	return nil
}

type orphan struct {
	kind, id, uuid string
}

// pruneOrphans deletes (--prune) or just reports state-tracked resources absent
// from the applied docs. Returns counts of deleted, would-delete, and skipped.
func pruneOrphans(rc *reconciler, docs []document, prune, dryRun bool, out io.Writer, errCount *int) (deleted, plannedDelete, skipped int) {
	applied := map[string]bool{}
	for _, d := range docs {
		applied[d.kind+"|"+d.id] = true
	}

	var orphans []orphan
	for kind, ids := range rc.state.Resources {
		for id, uuid := range ids {
			if !applied[kind+"|"+id] {
				orphans = append(orphans, orphan{kind: kind, id: id, uuid: uuid})
			}
		}
	}
	if len(orphans) == 0 {
		return 0, 0, 0
	}

	// Delete in reverse dependency order (rules before the targets they reference).
	sort.Slice(orphans, func(i, j int) bool {
		if oi, oj := kindOrder(orphans[i].kind), kindOrder(orphans[j].kind); oi != oj {
			return oi > oj
		}
		return orphans[i].id < orphans[j].id
	})

	for _, o := range orphans {
		label := o.kind + "/" + o.id
		switch {
		case !prune:
			fmt.Fprintf(out, "%s\torphaned (skipped)\n", label)
			skipped++
		case dryRun:
			fmt.Fprintf(out, "%s\twould delete\n", label)
			plannedDelete++
		default:
			base := kindBasePath(o.kind)
			if base == "" {
				fmt.Fprintf(out, "%s\terror: cannot prune unknown kind %q\n", label, o.kind)
				*errCount++
				continue
			}
			if err := rc.cl.delete(base + "/" + o.uuid); err != nil {
				fmt.Fprintf(out, "%s\terror: %v\n", label, err)
				*errCount++
				continue
			}
			rc.state.remove(o.kind, o.id)
			fmt.Fprintf(out, "%s\tdeleted\n", label)
			deleted++
		}
	}
	return deleted, plannedDelete, skipped
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

// kindBasePath maps a kind to its REST collection path; "" if unknown.
func kindBasePath(kind string) string {
	switch kind {
	case kindPolicy:
		return "/api/v1/policies"
	case kindSchedule:
		return "/api/v1/schedules"
	case kindAlertTarget:
		return "/api/v1/alert-targets"
	case kindAlertRule:
		return "/api/v1/alert-rules"
	case kindYaraSource:
		return "/api/v1/yara/signature-sources"
	default:
		return ""
	}
}

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

type document struct {
	kind   string
	id     string
	name   string
	source string

	policy      *policyDoc
	schedule    *scheduleDoc
	alertTarget *alertTargetDoc
	alertRule   *alertRuleDoc
	yaraSource  *yaraSourceDoc
}

// spec returns the active typed sub-document for validation.
func (d document) spec() any {
	switch d.kind {
	case kindPolicy:
		return d.policy
	case kindSchedule:
		return d.schedule
	case kindAlertTarget:
		return d.alertTarget
	case kindAlertRule:
		return d.alertRule
	case kindYaraSource:
		return d.yaraSource
	default:
		return nil
	}
}

// identifier is the stable handle (`id`), falling back to the display name only
// when reporting a missing id.
func (d document) identifier() string {
	if strings.TrimSpace(d.id) != "" {
		return d.id
	}
	return d.displayName()
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
	ID          string   `yaml:"id" validate:"required,handle"`
	Name        string   `yaml:"name" validate:"required"`
	Description string   `yaml:"description"`
	Query       string   `yaml:"query" validate:"required"`
	Platform    string   `yaml:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Resolution  string   `yaml:"resolution"`
	Enabled     *bool    `yaml:"enabled"`
	Groups      []string `yaml:"groups"`
}

type scheduleDoc struct {
	Kind        string   `yaml:"kind"`
	ID          string   `yaml:"id" validate:"required,handle"`
	Name        string   `yaml:"name" validate:"required"`
	Description string   `yaml:"description"`
	Query       string   `yaml:"query" validate:"required"`
	Interval    int      `yaml:"interval" validate:"gte=1,lte=604800"`
	Platform    string   `yaml:"platform" validate:"omitempty,oneof=darwin linux posix windows any all"`
	Snapshot    bool     `yaml:"snapshot"`
	Removed     bool     `yaml:"removed"`
	Shard       int      `yaml:"shard" validate:"gte=0,lte=100"`
	Version     string   `yaml:"version"`
	Groups      []string `yaml:"groups"`
}

type alertTargetDoc struct {
	Kind    string         `yaml:"kind"`
	ID      string         `yaml:"id" validate:"required,handle"`
	Name    string         `yaml:"name" validate:"required"`
	Type    string         `yaml:"type" validate:"required,oneof=smtp webhook"`
	Enabled *bool          `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}

type alertRuleDoc struct {
	Kind               string         `yaml:"kind"`
	ID                 string         `yaml:"id" validate:"required,handle"`
	Name               string         `yaml:"name" validate:"required"`
	Description        string         `yaml:"description"`
	Source             string         `yaml:"source" validate:"required"`
	Severity           string         `yaml:"severity" validate:"required,oneof=critical high medium low info"`
	Enabled            *bool          `yaml:"enabled"`
	EvaluationInterval int            `yaml:"evaluation_interval" validate:"gte=60"`
	For                int            `yaml:"for" validate:"gte=0"`
	RepeatInterval     int            `yaml:"repeat_interval" validate:"gte=0"`
	Params             map[string]any `yaml:"params"`
	Targets            []string       `yaml:"targets"`
}

type yaraSourceDoc struct {
	Kind    string `yaml:"kind"`
	ID      string `yaml:"id" validate:"required,handle"`
	Name    string `yaml:"name"`
	URL     string `yaml:"url" validate:"required"`
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
			doc.id, doc.name, doc.policy = v.ID, v.Name, &v
		case kindSchedule:
			var v scheduleDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: schedule: %w", source, err)
			}
			doc.id, doc.name, doc.schedule = v.ID, v.Name, &v
		case kindAlertTarget:
			var v alertTargetDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: alert_target: %w", source, err)
			}
			doc.id, doc.name, doc.alertTarget = v.ID, v.Name, &v
		case kindAlertRule:
			var v alertRuleDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: alert_rule: %w", source, err)
			}
			doc.id, doc.name, doc.alertRule = v.ID, v.Name, &v
		case kindYaraSource:
			var v yaraSourceDoc
			if err := strictDecode(raw, &v); err != nil {
				return nil, fmt.Errorf("%s: yara_source: %w", source, err)
			}
			doc.id, doc.name, doc.yaraSource = v.ID, v.Name, &v
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

// validHandle constrains the `id` handle: letters, digits, hyphen, underscore.
var validHandle = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func newDocValidator() *validator.Validate {
	v := validator.New()
	_ = v.RegisterValidation("handle", func(fl validator.FieldLevel) bool {
		return validHandle.MatchString(fl.Field().String())
	})
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		return strings.SplitN(f.Tag.Get("yaml"), ",", 2)[0]
	})
	return v
}

// validateDocument validates a document's spec against its `validate` tags.
func validateDocument(v *validator.Validate, d document) error {
	if err := v.Struct(d.spec()); err != nil {
		return fmt.Errorf("%s: %s/%s: %s", d.source, d.kind, d.identifier(), formatValidationErrors(err))
	}
	return nil
}

// formatValidationErrors renders validator errors as "<field> (<rule>)", joined.
func formatValidationErrors(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	msgs := make([]string, 0, len(ve))
	for _, e := range ve {
		msgs = append(msgs, fmt.Sprintf("%s (%s)", e.Field(), e.Tag()))
	}
	return strings.Join(msgs, "; ")
}

type reconciler struct {
	cl     *apiClient
	state  *stateFile
	dryRun bool

	groupCache  map[string]string
	targetCache map[string]string
	yaraSources []yaraRemoteSource
	yaraLoaded  bool
}

func newReconciler(cl *apiClient, state *stateFile, dryRun bool) *reconciler {
	return &reconciler{
		cl:          cl,
		state:       state,
		dryRun:      dryRun,
		groupCache:  map[string]string{},
		targetCache: map[string]string{},
	}
}

// resolveUUID resolves a resource's UUID: state file first, then adopt an
// existing one by name (bootstrap). found is false when it must be created.
func (rc *reconciler) resolveUUID(kind, id, byNamePath, name string) (uuid string, found bool, err error) {
	if u, ok := rc.state.get(kind, id); ok {
		return u, true, nil
	}
	u, ok, err := rc.cl.lookupUUID(byNamePath, name)
	if err != nil {
		return "", false, err
	}
	if ok {
		rc.state.set(kind, id, u) // adopt
		return u, true, nil
	}
	return "", false, nil
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
	uuid, found, err := rc.resolveUUID(kindPolicy, p.ID, "/api/v1/policies/by-name/", p.Name)
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
	return rc.write(kindPolicy, p.ID, "/api/v1/policies", uuid, found, body)
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
	uuid, found, err := rc.resolveUUID(kindSchedule, s.ID, "/api/v1/schedules/by-name/", s.Name)
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
	return rc.write(kindSchedule, s.ID, "/api/v1/schedules", uuid, found, body)
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
	uuid, found, err := rc.resolveUUID(kindAlertTarget, t.ID, "/api/v1/alert-targets/by-name/", t.Name)
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
	newUUID, err := rc.cl.create("/api/v1/alert-targets", alertTargetCreatePayload{Name: t.Name, Type: t.Type, Config: cfg, Enabled: t.Enabled})
	if err != nil {
		return "", err
	}
	rc.state.set(kindAlertTarget, t.ID, newUUID)
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
	uuid, found, err := rc.resolveUUID(kindAlertRule, r.ID, "/api/v1/alert-rules/by-name/", r.Name)
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
	return rc.write(kindAlertRule, r.ID, "/api/v1/alert-rules", uuid, found, body)
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

	// YARA sources have no by-name endpoint, so adopt by their (url, group) key.
	uuid, found := rc.state.get(kindYaraSource, y.ID)
	if !found {
		sources, err := rc.yaraList()
		if err != nil {
			return "", err
		}
		if u, ok := matchYaraSource(sources, y.URL, groupID); ok {
			uuid, found = u, true
			rc.state.set(kindYaraSource, y.ID, u)
		}
	}

	body := yaraSourcePayload{GroupID: groupID, URL: y.URL, Label: y.Name, Enabled: y.Enabled}
	return rc.write(kindYaraSource, y.ID, "/api/v1/yara/signature-sources", uuid, found, body)
}

func matchYaraSource(sources []yaraRemoteSource, url, groupID string) (string, bool) {
	for _, s := range sources {
		if s.URL == url && s.GroupID == groupID {
			return s.UUID, true
		}
	}
	return "", false
}

// write creates or updates a resource whose POST and PUT bodies are identical,
// recording the new UUID in state on create.
func (rc *reconciler) write(kind, id, basePath, uuid string, exists bool, body any) (string, error) {
	if rc.dryRun {
		return planAction(exists), nil
	}
	if exists {
		if err := rc.cl.put(basePath+"/"+uuid, body); err != nil {
			return "", err
		}
		return actionConfigured, nil
	}
	newUUID, err := rc.cl.create(basePath, body)
	if err != nil {
		return "", err
	}
	rc.state.set(kind, id, newUUID)
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

const (
	stateFileVersion = 1
	defaultStatePath = "checkpost.state.json"
)

// stateFile maps each resource's stable handle (its YAML `id`) to the server-assigned UUID
type stateFile struct {
	Version   int                          `json:"version"`
	Resources map[string]map[string]string `json:"resources"`
}

// loadState reads the state file; a missing file yields empty state.
func loadState(path string) (*stateFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &stateFile{Version: stateFileVersion, Resources: map[string]map[string]string{}}, nil
		}
		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}
	var s stateFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}
	if s.Version != 0 && s.Version != stateFileVersion {
		return nil, fmt.Errorf("state file %s has unsupported version %d (want %d)", path, s.Version, stateFileVersion)
	}
	s.Version = stateFileVersion
	if s.Resources == nil {
		s.Resources = map[string]map[string]string{}
	}
	return &s, nil
}

func (s *stateFile) save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write state file %s: %w", path, err)
	}
	return nil
}

func (s *stateFile) get(kind, id string) (string, bool) {
	uuid, ok := s.Resources[kind][id]
	return uuid, ok
}

func (s *stateFile) set(kind, id, uuid string) {
	if s.Resources[kind] == nil {
		s.Resources[kind] = map[string]string{}
	}
	s.Resources[kind][id] = uuid
}

func (s *stateFile) remove(kind, id string) {
	delete(s.Resources[kind], id)
	if len(s.Resources[kind]) == 0 {
		delete(s.Resources, kind)
	}
}
