package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookRuleURL(t *testing.T) {
	const uuid = "b97c90c4-34a2-444a-a287-40f98a9db4d8"
	for _, tc := range []struct {
		name    string
		rootURL string
		uuid    string
		want    string
	}{
		{"root", "https://checkpost.example.com", uuid, "https://checkpost.example.com/alerts?rule=" + uuid},
		{"trailing slash", "https://checkpost.example.com/", uuid, "https://checkpost.example.com/alerts?rule=" + uuid},
		{"http and port", "http://localhost:1323", uuid, "http://localhost:1323/alerts?rule=" + uuid},
		{"query and fragment", "https://example.com/?old=value#section", uuid, "https://example.com/alerts?rule=" + uuid},
		{"escaped rule", "https://example.com", "rule & other", "https://example.com/alerts?rule=rule+%26+other"},
		{"missing root", "", uuid, ""},
		{"missing UUID", "https://example.com", "", ""},
		{"malformed root", "://invalid", uuid, ""},
		{"relative root", "/checkpost", uuid, ""},
		{"unsupported scheme", "ftp://example.com", uuid, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notifier := NewWebhookNotifier(tc.rootURL)
			if got := notifier.ruleURL(tc.uuid); got != tc.want {
				t.Fatalf("rule URL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWebhookPayloadOmitsEmptyRuleURL(t *testing.T) {
	payload := buildWebhookPayload(EventFiring, Rule{UUID: "test-rule"}, nil, "")
	body, err := json.Marshal(payload.Rule)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["url"]; present {
		t.Fatal("empty rule URL should be omitted")
	}
}

func TestWebhookSendIncludesRuleURL(t *testing.T) {
	for _, kind := range []EventKind{EventFiring, EventResolved} {
		t.Run(string(kind), func(t *testing.T) {
			received := make(chan webhookPayload, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("unexpected request: %s, Content-Type %q", r.Method, r.Header.Get("Content-Type"))
				}
				var payload webhookPayload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				received <- payload
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			config, err := json.Marshal(webhookConfig{URL: server.URL})
			if err != nil {
				t.Fatal(err)
			}
			rule := Rule{UUID: "b97c90c4-34a2-444a-a287-40f98a9db4d8", Name: "Machine offline", Source: "machine_offline", Severity: "high"}
			alerts := []Alert{
				{Key: "one", Labels: map[string]string{"host": "one"}, Annotations: map[string]string{"summary": "Machine offline"}},
				{Key: "two", Labels: map[string]string{"host": "two"}, Annotations: map[string]string{"summary": "Machine offline"}},
			}
			notifier := NewWebhookNotifier("https://checkpost.example.com/")
			if err := notifier.Send(context.Background(), kind, Target{Config: config}, alerts, rule); err != nil {
				t.Fatal(err)
			}
			if len(received) != 2 {
				t.Fatalf("received %d requests, want 2", len(received))
			}
			for range 2 {
				payload := <-received
				if payload.Rule.URL != "https://checkpost.example.com/alerts?rule="+rule.UUID {
					t.Errorf("unexpected rule URL: %q", payload.Rule.URL)
				}
				if payload.Rule.UUID != rule.UUID || payload.Rule.Name != rule.Name || payload.Rule.Source != rule.Source || payload.Rule.Severity != rule.Severity {
					t.Errorf("unexpected rule details: %+v", payload.Rule)
				}
				if payload.Kind != kind || payload.Status != kindLabel(kind) {
					t.Errorf("unexpected event: %q, %q", payload.Kind, payload.Status)
				}
				if len(payload.Alerts) != 1 || payload.Alerts[0].Summary != "Machine offline" || payload.Alerts[0].Key != payload.Alerts[0].Labels["host"] {
					t.Errorf("unexpected host group: %+v", payload.Alerts)
				}
			}
		})
	}
}
