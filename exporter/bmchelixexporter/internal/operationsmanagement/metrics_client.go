// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
)

// MetricsClient is responsible for sending the metrics payload to BMC Helix Operations Management
type MetricsClient struct {
	url        string
	httpClient *http.Client
	apiKey     configopaque.String
	logger     *zap.Logger
}

// NewMetricsClient creates a new MetricsClient
func NewMetricsClient(ctx context.Context, clientConfig confighttp.ClientConfig, apiKey configopaque.String, host component.Host, settings component.TelemetrySettings, logger *zap.Logger) (*MetricsClient, error) {
	httpClient, err := clientConfig.ToClient(ctx, host, settings)
	if err != nil {
		return nil, err
	}
	return &MetricsClient{
		url:        clientConfig.Endpoint + "/metrics-gateway-service/api/v1.0/insert",
		httpClient: httpClient,
		apiKey:     apiKey,
		logger:     logger,
	}, nil
}

// SendHelixPayload sends the metrics payload to BMC Helix Operations Management
func (mc *MetricsClient) SendHelixPayload(ctx context.Context, payload []BMCHelixOMMetric) error {
	if len(payload) == 0 {
		mc.logger.Warn("Payload is empty, nothing to send")
		return nil
	}

	// Log the payload being sent
	mc.logger.Debug("Sending payload to BMC Helix Operations Management", zap.Any("payload", payload))

	// Get the JSON encoded payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		mc.logger.Error("Failed to marshal metrics payload", zap.Error(err))
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create a new HTTP request to send the payload
	req, err := mc.createNewHTTPRequest(ctx, payloadBytes)
	if err != nil {
		return err
	}

	// Send the request
	resp, err := mc.httpClient.Do(req)
	if err != nil {
		mc.logger.Error("Failed to send request to BMC Helix Operations Management", zap.Error(err))
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		mc.logger.Error("Received non-2xx response from BMC Helix Operations Management", zap.Int("status_code", resp.StatusCode))
		return fmt.Errorf("received non-2xx response: %d", resp.StatusCode)
	}

	mc.logger.Debug("Successfully sent payload to BMC Helix Operations Management", zap.String("url", mc.url))
	return nil
}

// createNewHTTPRequest creates a new HTTP request with the payload
func (mc *MetricsClient) createNewHTTPRequest(ctx context.Context, payloadBytes []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mc.url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		mc.logger.Error("Failed to create HTTP request", zap.Error(err))
		return nil, err
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(mc.apiKey))

	return req, nil
}
