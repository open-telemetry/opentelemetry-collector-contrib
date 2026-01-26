// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/alibaba/ecs"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	metadataBaseURL    = "http://100.100.100.200/latest/meta-data/"
	tokenURL           = "http://100.100.100.200/latest/api/token" //nolint:gosec // not a credential, metadata service URL
	tokenTTLHeader     = "X-aliyun-ecs-metadata-token-ttl-seconds" //nolint:gosec // not a credential, header name
	tokenHeader        = "X-aliyun-ecs-metadata-token"             //nolint:gosec // not a credential, header name
	defaultTokenTTL    = 21600
	tokenRefreshBuffer = 5 * time.Minute
	maxResponseSize    = 1 << 20 // 1 MB limit for metadata responses
)

// Metadata represents Alibaba Cloud ECS instance metadata.
type Metadata struct {
	Hostname       string
	ImageID        string
	InstanceID     string
	InstanceType   string
	OwnerAccountID string
	RegionID       string
	ZoneID         string
}

// Provider is the interface for retrieving Alibaba Cloud ECS metadata.
type Provider interface {
	// Metadata retrieves all ECS instance metadata.
	Metadata(ctx context.Context) (*Metadata, error)
}

type metadataClient struct {
	client *http.Client

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

var _ Provider = (*metadataClient)(nil)

// NewProvider returns a new Alibaba Cloud ECS metadata provider.
func NewProvider() Provider {
	return &metadataClient{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// getToken retrieves a metadata token, refreshing if necessary.
func (c *metadataClient) getToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Return cached token if still valid
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, tokenURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set(tokenTTLHeader, strconv.Itoa(defaultTokenTTL))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get metadata token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return "", fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	tokenBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	c.token = string(tokenBytes)
	c.tokenExpiry = time.Now().Add(time.Duration(defaultTokenTTL)*time.Second - tokenRefreshBuffer)

	return c.token, nil
}

// getMetadata retrieves a single metadata value from the given path.
func (c *metadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	fullURL := metadataBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request for %s: %w", path, err)
	}
	req.Header.Set(tokenHeader, token)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get metadata %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return "", fmt.Errorf("metadata %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read metadata %s: %w", path, err)
	}

	return string(data), nil
}

// Metadata retrieves all ECS instance metadata.
func (c *metadataClient) Metadata(ctx context.Context) (*Metadata, error) {
	// Fetch all metadata fields
	hostname, err := c.getMetadata(ctx, "hostname")
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	imageID, err := c.getMetadata(ctx, "image-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get image-id: %w", err)
	}

	instanceID, err := c.getMetadata(ctx, "instance-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance-id: %w", err)
	}

	instanceType, err := c.getMetadata(ctx, "instance/instance-type")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance-type: %w", err)
	}

	ownerAccountID, err := c.getMetadata(ctx, "owner-account-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get owner-account-id: %w", err)
	}

	regionID, err := c.getMetadata(ctx, "region-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get region-id: %w", err)
	}

	zoneID, err := c.getMetadata(ctx, "zone-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get zone-id: %w", err)
	}

	return &Metadata{
		Hostname:       hostname,
		ImageID:        imageID,
		InstanceID:     instanceID,
		InstanceType:   instanceType,
		OwnerAccountID: ownerAccountID,
		RegionID:       regionID,
		ZoneID:         zoneID,
	}, nil
}
