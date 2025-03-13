// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// faroExporter is an exporter that sends data to a Faro endpoint.
type faroExporter struct {
	url            string
	client         *http.Client
	clientSettings *confighttp.ClientConfig
	settings       component.TelemetrySettings
}

func createFaroExporter(cfg *Config, settings component.TelemetrySettings) (*faroExporter, error) {
	fe := &faroExporter{
		url:            cfg.Endpoint,
		clientSettings: &cfg.ClientConfig,
		client:         nil,
		settings:       settings,
	}

	return fe, nil
}

// start creates the http client
func (fe *faroExporter) start(ctx context.Context, host component.Host) (err error) {
	fe.client, err = fe.clientSettings.ToClient(ctx, host, fe.settings)
	return
}

func (fe *faroExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	// TODO: Implement conversion from traces to Faro format
	body, err := json.Marshal(map[string]interface{}{
		"message": "Traces data would be sent here",
	})
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to marshal traces data: %w", err))
	}

	return fe.sendToFaro(ctx, body)
}

func (fe *faroExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	// TODO: Implement conversion from logs to Faro format
	body, err := json.Marshal(map[string]interface{}{
		"message": "Logs data would be sent here",
	})
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to marshal logs data: %w", err))
	}

	return fe.sendToFaro(ctx, body)
}

func (fe *faroExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	// TODO: Implement conversion from metrics to Faro format
	body, err := json.Marshal(map[string]interface{}{
		"message": "Metrics data would be sent here",
	})
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to marshal metrics data: %w", err))
	}

	return fe.sendToFaro(ctx, body)
}

func (fe *faroExporter) sendToFaro(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fe.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := fe.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send data to Faro: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed the request with status code %d", resp.StatusCode)
	}
	return nil
}
