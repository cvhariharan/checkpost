package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiClient is the thin bearer-auth REST client used by `checkpost apply`. It
// targets <server>/api/v1/... and refuses to send the token over plain http to
// a non-loopback host unless insecure is set.
type apiClient struct {
	baseURL  string
	token    string
	insecure bool
	http     *http.Client
}

func newAPIClient(server, token string, insecure bool) (*apiClient, error) {
	server = strings.TrimRight(strings.TrimSpace(server), "/")
	if server == "" {
		return nil, errors.New("server URL is empty")
	}

	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL %q: %w", server, err)
	}
	switch u.Scheme {
	case "http":
		if !insecure && !isLoopbackHost(u.Hostname()) {
			return nil, fmt.Errorf("refusing to send the API token over plain http to non-loopback host %q; use https or pass --insecure", u.Host)
		}
	case "https":
	default:
		return nil, fmt.Errorf("server URL must be http or https, got %q", server)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	if insecure {
		// --insecure also skips TLS certificate verification, so the CLI works
		// against a server using a self-signed cert. Intended for dev/internal
		// hosts; production should use a trusted cert and drop --insecure.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		httpClient.Transport = transport
	}

	return &apiClient{
		baseURL:  server,
		token:    token,
		insecure: insecure,
		http:     httpClient,
	}, nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// apiError carries the server's status and (decoded) error message.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string {
	if e.msg == "" {
		return fmt.Sprintf("server returned %d", e.status)
	}
	return fmt.Sprintf("server returned %d: %s", e.status, e.msg)
}

func (e *apiError) notFound() bool { return e.status == http.StatusNotFound }

func (cl *apiClient) get(path string, out any) error  { return cl.do(http.MethodGet, path, nil, out) }
func (cl *apiClient) put(path string, body any) error { return cl.do(http.MethodPut, path, body, nil) }
func (cl *apiClient) delete(path string) error        { return cl.do(http.MethodDelete, path, nil, nil) }

// create POSTs body and returns the new resource's UUID from the {"id":...} response.
func (cl *apiClient) create(path string, body any) (string, error) {
	var res struct {
		ID string `json:"id"`
	}
	if err := cl.do(http.MethodPost, path, body, &res); err != nil {
		return "", err
	}
	if res.ID == "" {
		return "", fmt.Errorf("create %s: server returned no id", path)
	}
	return res.ID, nil
}

func (cl *apiClient) do(method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, cl.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cl.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{status: resp.StatusCode, msg: serverErrorMessage(data)}
	}

	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// serverErrorMessage extracts the {"error": "..."} message the server emits via
// its ErrorHandler, falling back to the raw body.
func serverErrorMessage(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ""
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(trimmed, &payload); err == nil && payload.Error != "" {
		return payload.Error
	}
	return string(trimmed)
}

// lookupUUID resolves a natural-key name to a UUID via a by-name endpoint.
// byNamePath must end in "/" (e.g. "/api/v1/policies/by-name/"). A 404 returns
// found=false with a nil error.
func (cl *apiClient) lookupUUID(byNamePath, name string) (uuid string, found bool, err error) {
	var res struct {
		UUID string `json:"uuid"`
	}
	if err := cl.get(byNamePath+url.PathEscape(name), &res); err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.notFound() {
			return "", false, nil
		}
		return "", false, err
	}
	return res.UUID, true, nil
}

// yaraRemoteSource is the subset of a YARA signature source the reconciler needs
// to match on the (url + group) natural key.
type yaraRemoteSource struct {
	UUID    string `json:"uuid"`
	GroupID string `json:"group_id"`
	URL     string `json:"url"`
}

// listYaraSources fetches all YARA signature sources, paging until exhausted.
func (cl *apiClient) listYaraSources() ([]yaraRemoteSource, error) {
	const perPage = 200
	var all []yaraRemoteSource
	for page := 1; ; page++ {
		var resp struct {
			Sources   []yaraRemoteSource `json:"sources"`
			PageCount int                `json:"page_count"`
		}
		path := fmt.Sprintf("/api/v1/yara/signature-sources?page=%d&count_per_page=%d", page, perPage)
		if err := cl.get(path, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Sources...)
		if page >= resp.PageCount || len(resp.Sources) == 0 {
			break
		}
	}
	return all, nil
}
