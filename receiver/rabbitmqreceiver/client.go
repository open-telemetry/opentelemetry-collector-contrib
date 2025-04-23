// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

const (
	// queuePath is the endpoint for RabbitMQ queues.
	queuePath = "/api/queues"

	// nodePath is the endpoint for RabbitMQ nodes.
	nodePath = "/api/nodes"
)

type client interface {
	// GetQueues calls "/api/queues" endpoint to get list of queues for the target node
	GetQueues(ctx context.Context) ([]*models.Queue, error)
	// GetNodes calls "/api/nodes" endpoint to get list of nodes for the target node
	GetNodes(ctx context.Context) ([]*models.Node, error)
}

var _ client = (*rabbitmqClient)(nil)

type rabbitmqClient struct {
	client       *http.Client
	hostEndpoint string
	creds        rabbitmqCredentials
	logger       *zap.Logger
}

type rabbitmqCredentials struct {
	username string
	password string
}

func newClient(ctx context.Context, cfg *Config, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (client, error) {
	httpClient, err := cfg.ToClient(ctx, host, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	return &rabbitmqClient{
		client:       httpClient,
		hostEndpoint: cfg.Endpoint,
		creds: rabbitmqCredentials{
			username: cfg.Username,
			password: string(cfg.Password),
		},
		logger: logger,
	}, nil
}

func (c *rabbitmqClient) GetQueues(ctx context.Context) ([]*models.Queue, error) {
	var queues []*models.Queue

	if err := c.get(ctx, queuePath, &queues); err != nil {
		c.logger.Debug("Failed to retrieve queues", zap.Error(err))
		return nil, err
	}

	return queues, nil
}

func (c *rabbitmqClient) GetNodes(ctx context.Context) ([]*models.Node, error) {
	var nodes []*models.Node

	if err := c.get(ctx, nodePath, &nodes); err != nil {
		c.logger.Debug("Failed to retrieve nodes", zap.Error(err))
		return nil, err
	}

	return nodes, nil
}

func (c *rabbitmqClient) get(ctx context.Context, path string, respObj any) error {
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
		c.logger.Debug("rabbitMQ API non-200", zap.Error(err), zap.Int("status_code", resp.StatusCode))

		// Attempt to extract the error payload
		payloadData, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Debug("failed to read payload error message", zap.Error(err))
		} else {
			c.logger.Debug("rabbitMQ API Error", zap.ByteString("api_error", payloadData))
		}

		return fmt.Errorf("non 200 code returned %d", resp.StatusCode)
	}

	// Decode the payload into the passed in response object
	if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
		return fmt.Errorf("failed to decode response payload: %w", err)
	}

	return nil
}
