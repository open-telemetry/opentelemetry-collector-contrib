// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/oraclecloud"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// OracleCloud IMDS compute endpoint
	metadataEndpoint = "http://169.254.169.254/opc/v2/instance/"
)

// Provider gets metadata from the OracleCloud IMDS.
type Provider interface {
	Metadata(context.Context) (*ComputeMetadata, error)
}

type oraclecloudProviderImpl struct {
	endpoint string
	client   *http.Client
}

// NewProvider creates a new metadata provider
func NewProvider() Provider {
	return &oraclecloudProviderImpl{
		endpoint: metadataEndpoint,
		client:   &http.Client{},
	}
}

type ComputeTagsListMetadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ComputeMetadata is the OracleCloud IMDS compute metadata response format
type ComputeMetadata struct {
	HostID             string `json:"id"`
	HostDisplayName    string `json:"displayName"`
	HostType           string `json:"shape"`
	RegionID           string `json:"canonicalRegionName"`
	AvailabilityDomain string `json:"availabilityDomain"`

	Metadata InstanceMetadata `json:"metadata"`
}

type InstanceMetadata struct {
	OKEClusterDisplayName string `json:"oke-cluster-display-name"`
}

// Metadata queries a given endpoint and parses the output to the OracleCloud IMDS format
func (p *oraclecloudProviderImpl) Metadata(ctx context.Context) (*ComputeMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer Oracle")
	q := req.URL.Query()
	req.URL.RawQuery = q.Encode()

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query OracleCloud IMDS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response from OracleCloud IMDS: %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OracleCloud IMDS reply: %w", err)
	}

	var metadata *ComputeMetadata
	err = json.Unmarshal(respBody, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to decode OracleCloud IMDS reply: %w", err)
	}

	return metadata, nil
}
