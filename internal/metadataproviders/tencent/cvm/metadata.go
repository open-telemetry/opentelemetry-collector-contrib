// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/tencent/cvm"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	metadataBaseURL = "http://metadata.tencentyun.com/latest/meta-data/"
	maxResponseSize = 1 << 20 // 1 MB limit for metadata responses
)

// Metadata represents Tencent Cloud CVM instance metadata.
type Metadata struct {
	InstanceName string
	ImageID      string
	InstanceID   string
	InstanceType string
	AppID        string
	RegionID     string
	ZoneID       string
}

// Provider is the interface for retrieving Tencent Cloud CVM metadata.
type Provider interface {
	// Metadata retrieves all CVM instance metadata.
	Metadata(ctx context.Context) (*Metadata, error)
}

type metadataClient struct {
	client *http.Client
}

var _ Provider = (*metadataClient)(nil)

// NewProvider returns a new Tencent Cloud CVM metadata provider.
func NewProvider() Provider {
	return &metadataClient{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// getMetadata retrieves a single metadata value from the given path.
func (c *metadataClient) getMetadata(ctx context.Context, path string) (string, error) {
	fullURL := metadataBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request for %s: %w", path, err)
	}

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

// Metadata retrieves all CVM instance metadata.
func (c *metadataClient) Metadata(ctx context.Context) (*Metadata, error) {
	instanceName, err := c.getMetadata(ctx, "instance-name")
	if err != nil {
		return nil, fmt.Errorf("failed to get instance-name: %w", err)
	}

	imageID, err := c.getMetadata(ctx, "instance/image-id")
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

	appID, err := c.getMetadata(ctx, "app-id")
	if err != nil {
		return nil, fmt.Errorf("failed to get app-id: %w", err)
	}

	regionID, err := c.getMetadata(ctx, "placement/region")
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	zoneID, err := c.getMetadata(ctx, "placement/zone")
	if err != nil {
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}

	return &Metadata{
		InstanceName: instanceName,
		ImageID:      imageID,
		InstanceID:   instanceID,
		InstanceType: instanceType,
		AppID:        appID,
		RegionID:     regionID,
		ZoneID:       zoneID,
	}, nil
}
