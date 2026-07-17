// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/vpc"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	metadataHost        = "api.metadata.cloud.ibm.com"
	tokenPath           = "/identity/v1/token"
	instancePath        = "/metadata/v1/instance"
	apiVersion          = "2026-01-30"
	metadataFlavorKey   = "Metadata-Flavor"
	metadataFlavorValue = "ibm"
	defaultTokenTTL     = 300 // seconds
	tokenRefreshBuffer  = 30 * time.Second
	maxResponseSize     = 1 << 20 // 1 MB limit for metadata responses
)

// Provider gathers IBM Cloud VPC instance metadata.
type Provider interface {
	InstanceMetadata(ctx context.Context) (*InstanceMetadata, error)
}

// InstanceMetadata represents the response from the VPC IMDS instance endpoint.
type InstanceMetadata struct {
	ID      string `json:"id"`
	CRN     string `json:"crn"`
	Name    string `json:"name"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	Zone struct {
		Name string `json:"name"`
	} `json:"zone"`
	VPC struct {
		ID   string `json:"id"`
		CRN  string `json:"crn"`
		Name string `json:"name"`
	} `json:"vpc"`
	ResourceGroup struct {
		ID string `json:"id"`
	} `json:"resource_group"`
	Image struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"image"`
}

// tokenResponse represents the IMDS token response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type metadataClient struct {
	endpoint string
	client   *http.Client

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

var _ Provider = (*metadataClient)(nil)

// NewProvider returns a new IBM Cloud VPC metadata provider.
// The protocol parameter selects the scheme ("http" or "https").
// Both schemes target the same host (api.metadata.cloud.ibm.com).
func NewProvider(protocol string) Provider {
	return newProvider(protocol + "://" + metadataHost)
}

// newProvider is the internal constructor that accepts an arbitrary endpoint.
// It is used by unit tests to point at httptest servers.
func newProvider(endpoint string) Provider {
	return &metadataClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout:   5 * time.Second,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
	}
}

// InstanceMetadata retrieves instance metadata from the IBM Cloud VPC IMDS.
func (c *metadataClient) InstanceMetadata(ctx context.Context) (*InstanceMetadata, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance identity token: %w", err)
	}

	return c.getInstanceMetadata(ctx, token)
}

// getToken retrieves an instance identity token, refreshing if necessary.
func (c *metadataClient) getToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Return cached token if still valid
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	url := fmt.Sprintf("%s%s?version=%s", c.endpoint, tokenPath, apiVersion)
	body := fmt.Appendf(nil, `{"expires_in":%d}`, defaultTokenTTL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set(metadataFlavorKey, metadataFlavorValue)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get metadata token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return "", fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(respBody))
	}

	var tr tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&tr); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	c.token = tr.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn)*time.Second - tokenRefreshBuffer)

	return c.token, nil
}

// getInstanceMetadata retrieves instance metadata using the given token.
func (c *metadataClient) getInstanceMetadata(ctx context.Context, token string) (*InstanceMetadata, error) {
	url := fmt.Sprintf("%s%s?version=%s", c.endpoint, instancePath, apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return nil, fmt.Errorf("instance metadata request returned %d: %s", resp.StatusCode, string(body))
	}

	var meta InstanceMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode instance metadata: %w", err)
	}

	return &meta, nil
}
