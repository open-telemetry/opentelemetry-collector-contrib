// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
)

// SentryAPIClient defines the interface for interacting with Sentry API
type SentryAPIClient interface {
	GetAllProjects(ctx context.Context, orgSlug string) ([]ProjectInfo, error)
	GetProjectKeys(ctx context.Context, orgSlug, projectSlug string) ([]ProjectKey, error)
	GetOrgProjectKeys(ctx context.Context, orgSlug string) ([]ProjectKey, error)
	GetOTLPEndpoints(ctx context.Context, orgSlug, projectSlug string) (*OTLPEndpoints, error)
	CreateProject(ctx context.Context, orgSlug, teamSlug, projectSlug, projectName, platform string) (*ProjectInfo, error)
}

// SentryClient handles communication with the Sentry API
type SentryClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// newSentryClient is used to override the sentry client factory. While running tests we need to mock the http transport, so that
// we don't open real sockets.
var newSentryClient = func(baseURL, authToken string, httpClient *http.Client) SentryAPIClient {
	return NewSentryClient(baseURL, authToken, httpClient)
}

// ProjectKey represents a Sentry project key
type ProjectKey struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Public    string   `json:"public"`
	Secret    string   `json:"secret"`
	ProjectID int      `json:"projectId"`
	IsActive  bool     `json:"isActive"`
	DSN       DSNField `json:"dsn"`
}

// DSNField represents the DSN field from API response
type DSNField struct {
	Public string `json:"public"`
}

// ParsePublicDSN parses the public DSN string into a sentry.Dsn
func (d *DSNField) ParsePublicDSN() (*sentry.Dsn, error) {
	return sentry.NewDsn(d.Public)
}

// TeamInfo represents a Sentry team
type TeamInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ProjectInfo represents a Sentry project
type ProjectInfo struct {
	ID    string     `json:"id"`
	Slug  string     `json:"slug"`
	Name  string     `json:"name"`
	Team  TeamInfo   `json:"team"`
	Teams []TeamInfo `json:"teams"`
}

// OTLPEndpoints contains the OTLP endpoint URLs for a project
type OTLPEndpoints struct {
	TracesURL string
	LogsURL   string
	PublicKey string
}

// SentryAPIError represents an error from Sentry Management API
type SentryAPIError struct {
	StatusCode int
	Body       string
	Headers    http.Header
}

func (e *SentryAPIError) Error() string {
	return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, e.Body)
}

func parseProjectID(id string) int {
	projectID, err := strconv.Atoi(id)
	if err != nil {
		return -1
	}
	return projectID
}

// NewSentryClient creates a new Sentry API client
func NewSentryClient(baseURL, authToken string, httpClient *http.Client) *SentryClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &SentryClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		authToken: authToken,
		client:    httpClient,
	}
}

// GetAllProjects fetches all projects for a given organization
func (c *SentryClient) GetAllProjects(ctx context.Context, orgSlug string) ([]ProjectInfo, error) {
	url := fmt.Sprintf("%s/api/0/organizations/%s/projects/", c.baseURL, orgSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer c.closeIdleConnections()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &SentryAPIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Headers:    resp.Header,
		}
	}

	var projects []ProjectInfo
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return projects, nil
}

// GetProjectKeys fetches the project keys for a given organization and project
func (c *SentryClient) GetProjectKeys(ctx context.Context, orgSlug, projectSlug string) ([]ProjectKey, error) {
	url := fmt.Sprintf("%s/api/0/projects/%s/%s/keys/", c.baseURL, orgSlug, projectSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer c.closeIdleConnections()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &SentryAPIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Headers:    resp.Header,
		}
	}

	var keys []ProjectKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return keys, nil
}

// GetOrgProjectKeys fetches all project keys for an organization
func (c *SentryClient) GetOrgProjectKeys(ctx context.Context, orgSlug string) ([]ProjectKey, error) {
	url := fmt.Sprintf("%s/api/0/organizations/%s/project-keys/", c.baseURL, orgSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer c.closeIdleConnections()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &SentryAPIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Headers:    resp.Header,
		}
	}

	var keys []ProjectKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return keys, nil
}

// CreateProject creates a new project in the given organization and team
func (c *SentryClient) CreateProject(ctx context.Context, orgSlug, teamSlug, projectSlug, projectName, platform string) (*ProjectInfo, error) {
	url := fmt.Sprintf("%s/api/0/teams/%s/%s/projects/", c.baseURL, orgSlug, teamSlug)

	payload := map[string]string{
		"name":     projectName,
		"slug":     projectSlug,
		"platform": platform,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer c.closeIdleConnections()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &SentryAPIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Headers:    resp.Header,
		}
	}

	var project ProjectInfo
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &project, nil
}

// GetOTLPEndpoints returns the OTLP endpoint URLs for a project
func (c *SentryClient) GetOTLPEndpoints(ctx context.Context, orgSlug, projectSlug string) (*OTLPEndpoints, error) {
	keys, err := c.GetProjectKeys(ctx, orgSlug, projectSlug)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no project keys found for project %s/%s", orgSlug, projectSlug)
	}

	var activeKey *ProjectKey
	for i := range keys {
		if keys[i].IsActive {
			activeKey = &keys[i]
			break
		}
	}

	if activeKey == nil {
		return nil, fmt.Errorf("no active project keys found for project %s/%s", orgSlug, projectSlug)
	}

	dsn, err := activeKey.DSN.ParsePublicDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	endpoints := &OTLPEndpoints{
		TracesURL: fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/traces/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
		LogsURL:   fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/logs/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
		PublicKey: activeKey.Public,
	}

	return endpoints, nil
}

// closeIdleConnections drains any idle connections created by the Sentry client.
func (c *SentryClient) closeIdleConnections() {
	if c == nil || c.client == nil {
		return
	}
	if tr, ok := c.client.Transport.(interface{ CloseIdleConnections() }); ok {
		tr.CloseIdleConnections()
	}
}
