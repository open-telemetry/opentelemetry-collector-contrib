// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/openstack/nova"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	metadataURL = "http://169.254.169.254/openstack/latest/meta_data.json"
)

type Provider interface {
	Get(ctx context.Context) (Document, error)
	Hostname(ctx context.Context) (string, error)
	InstanceID(ctx context.Context) (string, error)
}

type metadataClient struct {
	client *http.Client
}

var _ Provider = (*metadataClient)(nil)

// Document is a minimal representation of OpenStack's meta_data.json
type Document struct {
	UUID     string            `json:"uuid"`
	Hostname string            `json:"hostname"`
	Name     string            `json:"name"`
	Meta     map[string]string `json:"meta"`
}

// NewProvider returns a new Nova metadata provider with a short timeout.
func NewProvider() Provider {
	return &metadataClient{
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (c *metadataClient) getMetadata(ctx context.Context) (Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return Document{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return Document{}, fmt.Errorf("failed to query nova metadata service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Document{}, fmt.Errorf("metadata service returned %d: %s", resp.StatusCode, string(body))
	}

	var doc Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("failed to decode nova metadata: %w", err)
	}

	return doc, nil
}

func (c *metadataClient) InstanceID(ctx context.Context) (string, error) {
	doc, err := c.getMetadata(ctx)
	if err != nil {
		return "", err
	}
	if doc.UUID == "" {
		return "", fmt.Errorf("instance ID (uuid) not found in metadata")
	}
	return doc.UUID, nil
}

func (c *metadataClient) Hostname(ctx context.Context) (string, error) {
	doc, err := c.getMetadata(ctx)
	if err != nil {
		return "", err
	}
	if doc.Hostname == "" {
		return "", fmt.Errorf("hostname not found in metadata")
	}
	return doc.Hostname, nil
}

func (c *metadataClient) Get(ctx context.Context) (Document, error) {
	return c.getMetadata(ctx)
}
