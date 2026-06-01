package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type webhookConfig struct {
	URL string `json:"url" jsonschema:"title=URL,description=Webhook endpoint URL"`
}

type webhookPayloadOwner struct {
	Email       string `json:"email,omitempty"`
	ExternalID  string `json:"external_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type webhookPayloadRule struct {
	UUID        string `json:"uuid,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
}

type webhookPayloadAlert struct {
	Key         string            `json:"key"`
	Summary     string            `json:"summary,omitempty"`
	Resolution  string            `json:"resolution,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type webhookPayload struct {
	Kind   EventKind             `json:"kind"`
	Status string                `json:"status"`
	Rule   webhookPayloadRule    `json:"rule"`
	Owner  webhookPayloadOwner   `json:"owner,omitempty"`
	Alerts []webhookPayloadAlert `json:"alerts"`
}

// WebhookNotifier sends alert groups as JSON POST requests to per-target URLs.
type WebhookNotifier struct {
	client *http.Client
}

func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (WebhookNotifier) Type() string { return "webhook" }

func (WebhookNotifier) Schema() any { return reflectSchema(webhookConfig{}) }

func (WebhookNotifier) ValidateConfig(raw json.RawMessage) (json.RawMessage, error) {
	var c webhookConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	c.URL = strings.TrimSpace(c.URL)
	if c.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("url scheme must be http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	return json.Marshal(c)
}

func (w *WebhookNotifier) Send(ctx context.Context, kind EventKind, target Target, alerts []Alert, rule Rule) error {
	var c webhookConfig
	if err := json.Unmarshal(target.Config, &c); err != nil {
		return err
	}

	body, err := json.Marshal(buildWebhookPayload(kind, rule, alerts))
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "watcher-alerts")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("webhook returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func buildWebhookPayload(kind EventKind, rule Rule, alerts []Alert) webhookPayload {
	payload := webhookPayload{
		Kind:   kind,
		Status: kindLabel(kind),
		Rule: webhookPayloadRule{
			UUID:        rule.UUID,
			Name:        rule.Name,
			Description: rule.Description,
			Source:      rule.Source,
			Severity:    rule.Severity,
		},
		Owner:  ownerFromAlerts(alerts),
		Alerts: make([]webhookPayloadAlert, 0, len(alerts)),
	}
	for _, a := range alerts {
		payload.Alerts = append(payload.Alerts, webhookPayloadAlert{
			Key:         a.Key,
			Summary:     a.Annotations["summary"],
			Resolution:  a.Annotations["resolution"],
			Labels:      a.Labels,
			Annotations: a.Annotations,
		})
	}
	return payload
}

func ownerFromAlerts(alerts []Alert) webhookPayloadOwner {
	if len(alerts) == 0 {
		return webhookPayloadOwner{}
	}
	labels := alerts[0].Labels
	return webhookPayloadOwner{
		Email:       labels["owner_email"],
		ExternalID:  labels["owner_external_id"],
		DisplayName: labels["owner_display_name"],
	}
}
