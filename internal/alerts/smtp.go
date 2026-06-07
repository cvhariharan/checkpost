package alerts

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/knadh/smtppool/v2"
)

//go:embed templates/*.tmpl
var emailTemplates embed.FS

var (
	htmlTmpl = htmltemplate.Must(htmltemplate.ParseFS(emailTemplates, "templates/email.html.tmpl"))
	textTmpl = texttemplate.Must(texttemplate.ParseFS(emailTemplates, "templates/email.text.tmpl"))
)

type emailData struct {
	Title    string
	Kind     string
	Severity string
	Items    []emailItem
}

type emailItem struct {
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
}

func NewSMTPNotifier(relay SMTPRelay, resolveGroup GroupEmailResolver) (*SMTPNotifier, error) {
	n := &SMTPNotifier{from: relay.From, resolveGroup: resolveGroup}
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

	to, err := s.resolveRecipients(ctx, c.Recipients, alerts)
	if err != nil {
		return err
	}
	if len(to) == 0 {
		return nil
	}

	data := buildEmailData(kind, rule, alerts)
	text, err := render(textTmpl, data)
	if err != nil {
		return fmt.Errorf("render text body: %w", err)
	}
	html, err := render(htmlTmpl, data)
	if err != nil {
		return fmt.Errorf("render html body: %w", err)
	}

	return s.pool.Send(smtppool.Email{
		From:    s.from,
		To:      to,
		Subject: fmt.Sprintf("[%s] %s %s", strings.ToUpper(rule.Severity), rule.Name, kindLabel(kind)),
		Text:    text,
		HTML:    html,
	})
}

func (s *SMTPNotifier) resolveRecipients(ctx context.Context, recipients []string, alerts []Alert) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}

	for _, r := range recipients {
		r = strings.TrimSpace(r)
		switch {
		case r == "owner":
			if len(alerts) > 0 {
				add(alerts[0].Labels["owner_email"])
			}
		case strings.HasPrefix(r, "user-group:"):
			if s.resolveGroup == nil {
				continue
			}
			emails, err := s.resolveGroup(ctx, strings.TrimPrefix(r, "user-group:"))
			if err != nil {
				return nil, err
			}
			for _, e := range emails {
				add(e)
			}
		default:
			add(r)
		}
	}
	return out, nil
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
			Summary:    a.Annotations["summary"],
			Resolution: a.Annotations["resolution"],
		})
	}
	return d
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
