// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package couchbasereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchbasereceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// clusterPath is the path to cluster endpoint
const clusterPath = "/pools/default"

type client interface {
	// GetClusterDetails calls "/pools/default" endpoint to get the cluster details
	GetClusterDetails(ctx context.Context) (*clusterResponse, error)

	// GetBuckets calls the specified bucket path to return details on the buckets associated with the cluster.
	// The path should be extracted from the clusterResponse.BucketsInfo.URI
	GetBuckets(ctx context.Context, path string) ([]*bucket, error)

	// GetBucketStats retrieves the stats for the bucket at the given path.
	// The path should be extraced from bucket.StatsInfo.URI
	GetBucketStats(ctx context.Context, path string) (*bucketStats, error)
}

var _ client = (*couchbaseClient)(nil)

type couchbaseClient struct {
	client       *http.Client
	hostEndpoint string
	creds        couchbaseCredentials
	logger       *zap.Logger
}

type couchbaseCredentials struct {
	username string
	password string
}

func newClient(cfg *Config, host component.Host, settings component.TelemetrySettings) (client, error) {
	httpClient, err := cfg.ToClient(host.GetExtensions(), settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &couchbaseClient{
		client:       httpClient,
		hostEndpoint: cfg.Endpoint,
		creds: couchbaseCredentials{
			username: cfg.Username,
			password: cfg.Password,
		},
		logger: settings.Logger,
	}, nil
}

func (c *couchbaseClient) GetClusterDetails(ctx context.Context) (*clusterResponse, error) {
	var clusterInfo clusterResponse

	if err := c.get(ctx, clusterPath, &clusterInfo); err != nil {
		c.logger.Debug("Failed to retrieve cluster data", zap.Error(err))
		return nil, err
	}

	return &clusterInfo, nil
}

func (c *couchbaseClient) GetBuckets(ctx context.Context, path string) ([]*bucket, error) {
	buckets := make([]*bucket, 0)

	if err := c.get(ctx, path, &buckets); err != nil {
		c.logger.Debug("Failed to retrieve buckets", zap.Error(err), zap.String("path", path))
		return nil, err
	}
	return buckets, nil
}

func (c *couchbaseClient) GetBucketStats(ctx context.Context, path string) (*bucketStats, error) {
	var stats bucketStats

	if err := c.get(ctx, path, &stats); err != nil {
		c.logger.Debug("Failed to retrieve bucket stats", zap.Error(err), zap.String("path", path))
		return nil, err
	}
	return &stats, nil
}

func (c *couchbaseClient) get(ctx context.Context, path string, respObj interface{}) error {
	// Construct endpoint and create request
	url := c.hostEndpoint + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create get request for path %s: %w", path, err)
	}

	// Set user/pass authentication
	req.SetBasicAuth(c.creds.username, c.creds.password)

	// Make request
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make http request: %w", err)
	}

	// Defer body close
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	// Check for OK status code
	if resp.StatusCode != http.StatusOK {
		c.logger.Debug("couchbase API non-200", zap.Error(err), zap.Int("status_code", resp.StatusCode))

		// Attempt to extract the error payload
		payloadData, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Debug("failed to read payload error message", zap.Error(err))
		} else {
			c.logger.Debug("couchbase API Error", zap.ByteString("api_error", payloadData))
		}

		return fmt.Errorf("non 200 code returned %d", resp.StatusCode)
	}

	// Decode the payload into the passed in response object
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		return fmt.Errorf("failed to decode response payload: %w", err)
	}

	return nil
}
