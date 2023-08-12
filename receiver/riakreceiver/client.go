// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/model"
)

// statsPath is the path to stats endpoint
const statsPath = "/stats"

type client interface {
	// GetStats calls "/stats" endpoint to get list of stats for the target node
	GetStats(ctx context.Context) (*model.Stats, error)
}

var _ client = (*riakClient)(nil)

type riakClient struct {
	client       *http.Client
	hostEndpoint string
	creds        riakCredentials
	logger       *zap.Logger
}

type riakCredentials struct {
	username string
	password string
}

func newClient(cfg *Config, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (client, error) {
	httpClient, err := cfg.ToClient(host, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &riakClient{
		client:       httpClient,
		hostEndpoint: cfg.Endpoint,
		creds: riakCredentials{
			username: cfg.Username,
			password: string(cfg.Password),
		},
		logger: logger,
	}, nil
}

func (c *riakClient) GetStats(ctx context.Context) (*model.Stats, error) {
	var stats *model.Stats

	if err := c.get(ctx, statsPath, &stats); err != nil {
		c.logger.Debug("Failed to retrieve stats", zap.Error(err))
		return nil, err
	}

	return stats, nil
}

func (c *riakClient) get(ctx context.Context, path string, respObj interface{}) error {
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
		c.logger.Debug("riak API non-200", zap.Error(err), zap.Int("status_code", resp.StatusCode))

		// Attempt to extract the error payload
		payloadData, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Debug("failed to read payload error message", zap.Error(err))
		} else {
			c.logger.Debug("riak API Error", zap.ByteString("api_error", payloadData))
		}

		return fmt.Errorf("non 200 code returned %d", resp.StatusCode)
	}

	// Decode the payload into the passed in response object
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		return fmt.Errorf("failed to decode response payload: %w", err)
	}

	return nil
}
