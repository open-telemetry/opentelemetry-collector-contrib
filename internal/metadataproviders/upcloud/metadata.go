// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/upcloud"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Upcloud IMDS compute endpoint, see https://upcloud.com/docs/guides/upcloud-metadata-service/
	metadataEndpoint = "http://169.254.169.254/metadata/v1.json"
)

// Provider gets metadata from the Upcloud metadata service.
type Provider interface {
	Metadata(context.Context) (*Metadata, error)
}

type upcloudProviderImpl struct {
	endpoint string
	client   *http.Client
}

// NewProvider creates a new Upcloud metadata provider.
func NewProvider() Provider {
	return &upcloudProviderImpl{
		endpoint: metadataEndpoint,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
}

type Metadata struct {
	CloudName  string `json:"cloud_name"`
	Hostname   string `json:"hostname"`
	InstanceID string `json:"instance_id"`
	Region     string `json:"region"`
}

// Metadata fetches and decodes Upcloud instance metadata.
func (p *upcloudProviderImpl) Metadata(ctx context.Context) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query Upcloud metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Upcloud metadata replied with status code: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Upcloud metadata: %w", err)
	}

	var md Metadata
	if err := json.Unmarshal(body, &md); err != nil {
		return nil, fmt.Errorf("decode Upcloud metadata: %w", err)
	}
	return &md, nil
}
