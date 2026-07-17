// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/classic"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultEndpoint = "https://api.service.softlayer.com/rest/v3.1/SoftLayer_Resource_Metadata"
	maxResponseSize = 1 << 20 // 1 MB limit for metadata responses
)

// Provider gathers IBM Cloud Classic (SoftLayer) instance metadata.
type Provider interface {
	InstanceMetadata(ctx context.Context) (*InstanceMetadata, error)
}

// InstanceMetadata represents the metadata collected from the SoftLayer Resource Metadata API.
type InstanceMetadata struct {
	ID               string
	Hostname         string
	Datacenter       string
	AccountID        string
	GlobalIdentifier string
}

type metadataClient struct {
	endpoint string
	client   *http.Client
}

var _ Provider = (*metadataClient)(nil)

// NewProvider returns a new IBM Cloud Classic metadata provider
// that queries the SoftLayer Resource Metadata API.
func NewProvider() Provider {
	return newProvider(defaultEndpoint)
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

// InstanceMetadata retrieves instance metadata from the SoftLayer Resource Metadata API.
// It makes parallel GET requests for each metadata field using the plain-text (.txt)
// format, which returns unquoted values directly.
func (c *metadataClient) InstanceMetadata(ctx context.Context) (*InstanceMetadata, error) {
	g, ctx := errgroup.WithContext(ctx)
	var meta InstanceMetadata

	fetch := func(dst *string, method, label string) {
		g.Go(func() error {
			val, err := fetchText(ctx, c, method)
			if err != nil {
				return fmt.Errorf("failed to get %s: %w", label, err)
			}
			*dst = val
			return nil
		})
	}

	fetch(&meta.ID, "getId.txt", "instance ID")
	fetch(&meta.Hostname, "getHostname.txt", "hostname")
	fetch(&meta.Datacenter, "getDatacenter.txt", "datacenter")
	fetch(&meta.AccountID, "getAccountId.txt", "account ID")
	fetch(&meta.GlobalIdentifier, "getGlobalIdentifier.txt", "global identifier")

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &meta, nil
}

// fetchText makes a GET request to the given method path and returns the
// response body as a trimmed string. The SoftLayer metadata API .txt format
// returns plain-text values without JSON encoding.
func fetchText(ctx context.Context, c *metadataClient, method string) (string, error) {
	url := c.endpoint + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request for %s: %w", method, err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request for %s failed: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return "", fmt.Errorf("%s returned %d: %s", method, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response for %s: %w", method, err)
	}

	return strings.TrimSpace(string(body)), nil
}
