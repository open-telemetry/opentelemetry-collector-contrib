// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/vultr"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Vultr IMDS compute endpoint, see https://www.vultr.com/metadata/
	metadataEndpoint = "http://169.254.169.254/v1.json"
)

// Provider gets metadata from the Vultr metadata service.
type Provider interface {
	Metadata(context.Context) (*Metadata, error)
}

type vultrProviderImpl struct {
	endpoint string
	client   *http.Client
}

// NewProvider creates a new Vultr metadata provider.
func NewProvider() Provider {
	return &vultrProviderImpl{
		endpoint: metadataEndpoint,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
}

type Region struct {
	RegionCode string `json:"regioncode"`
}

type Metadata struct {
	Hostname     string `json:"hostname"`
	InstanceID   string `json:"instanceid"`
	InstanceV2ID string `json:"instance-v2-id"`
	Region       Region `json:"region"`
}

// Metadata fetches and decodes Vultr instance metadata.
func (p *vultrProviderImpl) Metadata(ctx context.Context) (*Metadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query Vultr metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Vultr metadata replied with status code: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Vultr metadata: %w", err)
	}

	var md Metadata
	if err := json.Unmarshal(body, &md); err != nil {
		return nil, fmt.Errorf("decode Vultr metadata: %w", err)
	}
	return &md, nil
}
