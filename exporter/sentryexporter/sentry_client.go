// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
)

// sentryAPIClient defines the interface for interacting with Sentry API.
type sentryAPIClient interface {
	GetAllProjects(ctx context.Context, orgSlug string) ([]projectInfo, error)
	GetProjectKeys(ctx context.Context, orgSlug, projectSlug string) ([]projectKey, error)
	GetOrgProjectKeys(ctx context.Context, orgSlug string) ([]projectKey, error)
	GetOTLPEndpoints(ctx context.Context, orgSlug, projectSlug string) (*otlpEndpoints, error)
	CreateProject(ctx context.Context, orgSlug, teamSlug, projectSlug, projectName, platform string) (*projectInfo, error)
}

// sentryClient handles communication with the Sentry API.
type sentryClient struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// newSentryClient is used to override the sentry client factory. While running tests we need to mock the http transport, so that
// we don't open real sockets.
var newSentryClient = func(baseURL, authToken string, httpClient *http.Client) sentryAPIClient {
	return newSentryClientImpl(baseURL, authToken, httpClient)
}

// projectKey represents a Sentry project key.
type projectKey struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Public    string   `json:"public"`
	Secret    string   `json:"secret"`
	ProjectID int      `json:"projectId"`
	IsActive  bool     `json:"isActive"`
	DSN       dsnField `json:"dsn"`
}

// dsnField represents the DSN field from API response.
type dsnField struct {
	Public string `json:"public"`
}

// ParsePublicDSN parses the public DSN string into a sentry.Dsn.
func (d *dsnField) ParsePublicDSN() (*sentry.Dsn, error) {
	return sentry.NewDsn(d.Public)
}

// teamInfo represents a Sentry team.
type teamInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// projectInfo represents a Sentry project.
type projectInfo struct {
	ID    string     `json:"id"`
	Slug  string     `json:"slug"`
	Name  string     `json:"name"`
	Team  teamInfo   `json:"team"`
	Teams []teamInfo `json:"teams"`
}

// otlpEndpoints contains the OTLP endpoint URLs for a project.
type otlpEndpoints struct {
	TracesURL  string
	LogsURL    string
	PublicKey  string
	AuthHeader string
}

func parseProjectID(id string) int {
	projectID, err := strconv.Atoi(id)
	if err != nil {
		return -1
	}
	return projectID
}

// parseNextCursor extracts the cursor for the next page from a Sentry Link header.
func parseNextCursor(header http.Header) (cursor string, hasMore, found bool) {
	linkHeader := header.Get("Link")
	if linkHeader == "" {
		return "", false, false
	}

	for part := range strings.SplitSeq(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}

		found = true
		hasMore = strings.Contains(part, `results="true"`)

		if idx := strings.Index(part, `cursor="`); idx != -1 {
			start := idx + len(`cursor="`)
			if end := strings.Index(part[start:], `"`); end != -1 {
				cursor = part[start : start+end]
			}
		}

		break
	}

	return cursor, hasMore, found
}

func newSentryClientImpl(baseURL, authToken string, httpClient *http.Client) *sentryClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &sentryClient{
		baseURL:   strings.TrimSuffix(baseURL, "/"),
		authToken: authToken,
		client:    httpClient,
	}
}

// GetAllProjects fetches all projects for a given organization.
func (c *sentryClient) GetAllProjects(ctx context.Context, orgSlug string) ([]projectInfo, error) {
	baseURL := fmt.Sprintf("%s/api/0/organizations/%s/projects/", c.baseURL, orgSlug)
	cursor := ""
	var projects []projectInfo

	for {
		pageURL := baseURL
		if cursor != "" {
			pageURL = fmt.Sprintf("%s?cursor=%s", baseURL, url.QueryEscape(cursor))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, &sentryHTTPError{
				statusCode: resp.StatusCode,
				body:       string(body),
				headers:    resp.Header,
			}
		}

		var pageProjects []projectInfo
		if err := json.NewDecoder(resp.Body).Decode(&pageProjects); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		nextCursor, hasMore, found := parseNextCursor(resp.Header)
		resp.Body.Close()

		projects = append(projects, pageProjects...)

		if !found || !hasMore || nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	return projects, nil
}

// GetProjectKeys fetches the project keys for a given organization and project.
func (c *sentryClient) GetProjectKeys(ctx context.Context, orgSlug, projectSlug string) ([]projectKey, error) {
	baseURL := fmt.Sprintf("%s/api/0/projects/%s/%s/keys/", c.baseURL, orgSlug, projectSlug)
	cursor := ""
	var keys []projectKey

	for {
		pageURL := baseURL
		if cursor != "" {
			pageURL = fmt.Sprintf("%s?cursor=%s", baseURL, url.QueryEscape(cursor))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, &sentryHTTPError{
				statusCode: resp.StatusCode,
				body:       string(body),
				headers:    resp.Header,
			}
		}

		var pageKeys []projectKey
		if err := json.NewDecoder(resp.Body).Decode(&pageKeys); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		nextCursor, hasMore, found := parseNextCursor(resp.Header)
		resp.Body.Close()

		keys = append(keys, pageKeys...)

		if !found || !hasMore || nextCursor == "" {
			break
		}

		cursor = nextCursor
	}

	return keys, nil
}

// GetOrgProjectKeys fetches all project keys for an organization.
func (c *sentryClient) GetOrgProjectKeys(ctx context.Context, orgSlug string) ([]projectKey, error) {
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &sentryHTTPError{
			statusCode: resp.StatusCode,
			body:       string(body),
			headers:    resp.Header,
		}
	}

	var keys []projectKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return keys, nil
}

// CreateProject creates a new project in the given organization and team.
func (c *sentryClient) CreateProject(ctx context.Context, orgSlug, teamSlug, projectSlug, projectName, platform string) (*projectInfo, error) {
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &sentryHTTPError{
			statusCode: resp.StatusCode,
			body:       string(respBody),
			headers:    resp.Header,
		}
	}

	var project projectInfo
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &project, nil
}

// GetOTLPEndpoints returns the OTLP endpoint URLs for a project.
func (c *sentryClient) GetOTLPEndpoints(ctx context.Context, orgSlug, projectSlug string) (*otlpEndpoints, error) {
	keys, err := c.GetProjectKeys(ctx, orgSlug, projectSlug)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no project keys found for project %s/%s", orgSlug, projectSlug)
	}

	var activeKey *projectKey
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

	endpoints := &otlpEndpoints{
		TracesURL:  fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/traces/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
		LogsURL:    fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/logs/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
		PublicKey:  activeKey.Public,
		AuthHeader: fmt.Sprintf("sentry sentry_key=%s", activeKey.Public),
	}

	return endpoints, nil
}
