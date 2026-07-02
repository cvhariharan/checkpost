package alerts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/knadh/smtppool/v2"
)

type emailData struct {
	Title    string
	Kind     string
	Severity string
	Items    []emailItem
}

type emailItem struct {
	Host       string
	Severity   string
	Summary    string
	Resolution string
}

var schemaReflector = &jsonschema.Reflector{ExpandedStruct: true, DoNotReference: true}

func reflectSchema(v any) *jsonschema.Schema { return schemaReflector.Reflect(v) }

type SMTPRelay struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLS      string // starttls | implicit | none
}

func (r SMTPRelay) Enabled() bool { return strings.TrimSpace(r.Host) != "" }

type smtpConfig struct {
	Recipients []string `json:"recipients" jsonschema:"title=Recipients,description=Plain addresses, user-group:<name>, or the owner token"`
}

// GroupEmailResolver resolves a user-group name to its members' emails.
type GroupEmailResolver func(ctx context.Context, name string) ([]string, error)

// SMTPNotifier sends one email per host group
type SMTPNotifier struct {
	from         string
	pool         *smtppool.Pool
	resolveGroup GroupEmailResolver
	htmlTmpl     *htmltemplate.Template
	textTmpl     *texttemplate.Template
}

// NewSMTPNotifier parses the email templates from the supplied filesystem and wires the SMTP connection pool
func NewSMTPNotifier(relay SMTPRelay, resolveGroup GroupEmailResolver, templates fs.FS) (*SMTPNotifier, error) {
	htmlTmpl, err := htmltemplate.ParseFS(templates, "email.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse html email template: %w", err)
	}
	textTmpl, err := texttemplate.ParseFS(templates, "email.text.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse text email template: %w", err)
	}

	n := &SMTPNotifier{from: relay.From, resolveGroup: resolveGroup, htmlTmpl: htmlTmpl, textTmpl: textTmpl}
	if !relay.Enabled() {
		return n, nil
	}

	opt := smtppool.Opt{
		Host:            relay.Host,
		Port:            relay.Port,
		MaxConns:        4,
		IdleTimeout:     10 * time.Second,
		PoolWaitTimeout: 10 * time.Second,
	}
	if relay.Username != "" {
		opt.Auth = &smtppool.LoginAuth{Username: relay.Username, Password: relay.Password}
	}
	switch relay.TLS {
	case "implicit":
		opt.SSL = smtppool.SSLTLS
		opt.TLSConfig = &tls.Config{ServerName: relay.Host}
	case "none":
		opt.SSL = smtppool.SSLNone
	default: // starttls
		opt.SSL = smtppool.SSLSTARTTLS
		opt.TLSConfig = &tls.Config{ServerName: relay.Host}
	}

	pool, err := smtppool.New(opt)
	if err != nil {
		return nil, fmt.Errorf("create smtp pool: %w", err)
	}
	n.pool = pool
	return n, nil
}

func (SMTPNotifier) Type() string { return "smtp" }

func (SMTPNotifier) Schema() any { return reflectSchema(smtpConfig{}) }

func (SMTPNotifier) ValidateConfig(raw json.RawMessage) (json.RawMessage, error) {
	var c smtpConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if len(c.Recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}
	return json.Marshal(c)
}

func (s *SMTPNotifier) Send(ctx context.Context, kind EventKind, target Target, alerts []Alert, rule Rule) error {
	if s.pool == nil {
		return fmt.Errorf("smtp relay not configured")
	}
	var c smtpConfig
	if err := json.Unmarshal(target.Config, &c); err != nil {
		return err
	}

	// Bundle alerts that share a recipient set into a single email
	batches, err := s.batchByRecipients(ctx, c.Recipients, alerts)
	if err != nil {
		return err
	}

	var errs []error
	for _, b := range batches {
		if err := s.sendBatch(kind, rule, b.recipients, b.alerts); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SMTPNotifier) sendBatch(kind EventKind, rule Rule, to []string, alerts []Alert) error {
	data := buildEmailData(kind, rule, alerts)
	text, err := render(s.textTmpl, data)
	if err != nil {
		return fmt.Errorf("render text body: %w", err)
	}
	html, err := render(s.htmlTmpl, data)
	if err != nil {
		return fmt.Errorf("render html body: %w", err)
	}

	subject := fmt.Sprintf("[%s] %s %s", strings.ToUpper(rule.Severity), rule.Name, kindLabel(kind))
	if len(alerts) > 1 {
		subject = fmt.Sprintf("%s (%d alerts)", subject, len(alerts))
	}

	return s.pool.Send(smtppool.Email{
		From:    s.from,
		To:      to,
		Subject: subject,
		Text:    text,
		HTML:    html,
	})
}

type emailBatch struct {
	recipients []string
	alerts     []Alert
}

func (s *SMTPNotifier) batchByRecipients(ctx context.Context, recipients []string, alerts []Alert) ([]emailBatch, error) {
	base, wantOwner, err := s.resolveStatic(ctx, recipients)
	if err != nil {
		return nil, err
	}

	var order []string
	batches := map[string]*emailBatch{}
	for _, a := range alerts {
		to := recipientsFor(base, wantOwner, a)
		if len(to) == 0 {
			continue
		}
		key := strings.Join(to, ",")
		b, ok := batches[key]
		if !ok {
			b = &emailBatch{recipients: to}
			batches[key] = b
			order = append(order, key)
		}
		b.alerts = append(b.alerts, a)
	}

	out := make([]emailBatch, 0, len(order))
	for _, k := range order {
		out = append(out, *batches[k])
	}
	return out, nil
}

func (s *SMTPNotifier) resolveStatic(ctx context.Context, recipients []string) (base []string, wantOwner bool, err error) {
	seen := map[string]struct{}{}
	add := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		base = append(base, addr)
	}

	for _, r := range recipients {
		r = strings.TrimSpace(r)
		switch {
		case r == "owner":
			wantOwner = true
		case strings.HasPrefix(r, "user-group:"):
			if s.resolveGroup == nil {
				continue
			}
			emails, err := s.resolveGroup(ctx, strings.TrimPrefix(r, "user-group:"))
			if err != nil {
				return nil, false, err
			}
			for _, e := range emails {
				add(e)
			}
		default:
			add(r)
		}
	}
	return base, wantOwner, nil
}

// recipientsFor returns the full recipient list for a single alert, appending
// its owner when requested and not already covered by the static set.
func recipientsFor(base []string, wantOwner bool, a Alert) []string {
	to := append([]string(nil), base...)
	if !wantOwner {
		return to
	}
	owner := strings.TrimSpace(a.Labels["owner_email"])
	if owner == "" {
		return to
	}
	for _, addr := range to {
		if addr == owner {
			return to
		}
	}
	return append(to, owner)
}

func kindLabel(k EventKind) string {
	if k == EventResolved {
		return "resolved"
	}
	return "firing"
}

func buildEmailData(kind EventKind, rule Rule, alerts []Alert) emailData {
	d := emailData{Title: rule.Name, Kind: kindLabel(kind), Severity: rule.Severity}
	for _, a := range alerts {
		d.Items = append(d.Items, emailItem{
			Host:       alertHost(a),
			Severity:   a.Labels["policy_severity"],
			Summary:    a.Annotations["summary"],
			Resolution: a.Annotations["resolution"],
		})
	}
	return d
}

func alertHost(a Alert) string {
	if h := strings.TrimSpace(a.Labels["hostname"]); h != "" {
		return h
	}
	return a.Labels["host"]
}

type executor interface {
	Execute(wr io.Writer, data any) error
}

func render(t executor, data emailData) ([]byte, error) {
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
