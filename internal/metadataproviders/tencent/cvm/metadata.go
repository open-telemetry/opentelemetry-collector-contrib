// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/tencent/cvm"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
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
	g, ctx := errgroup.WithContext(ctx)
	var md Metadata

	fetch := func(dst *string, key, label string) {
		g.Go(func() error {
			val, err := c.getMetadata(ctx, key)
			if err != nil {
				return fmt.Errorf("failed to get %s: %w", label, err)
			}
			*dst = val
			return nil
		})
	}

	fetch(&md.InstanceName, "instance-name", "instance-name")
	fetch(&md.ImageID, "instance/image-id", "image-id")
	fetch(&md.InstanceID, "instance-id", "instance-id")
	fetch(&md.InstanceType, "instance/instance-type", "instance-type")
	fetch(&md.AppID, "app-id", "app-id")
	fetch(&md.RegionID, "placement/region", "region")
	fetch(&md.ZoneID, "placement/zone", "zone")

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &md, nil
}
