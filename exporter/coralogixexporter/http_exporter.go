// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type httpExporter struct {
	Client   *http.Client
	Endpoint string
}

type httpLogsExporter struct {
	httpExporter
}

type httpMetricsExporter struct {
	httpExporter
}

type httpTracesExporter struct {
	httpExporter
}

// httpError wraps HTTP status codes, headers, and response body for error handling
type httpError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Message    string
}

func (e *httpError) Error() string {
	return e.Message
}

// ensureHTTPScheme ensures the endpoint has an https:// scheme
func ensureHTTPScheme(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "https://" + endpoint
	}
	return endpoint
}

func newHTTPLogsExporter(client *http.Client, config *Config) httpLogsExporter {
	endpoint := config.Logs.Endpoint
	if isEmpty(endpoint) {
		endpoint = ensureHTTPScheme(config.getDomainGrpcSettings().Endpoint)
	}

	return httpLogsExporter{
		httpExporter: httpExporter{
			Client:   client,
			Endpoint: endpoint,
		},
	}
}

func newHTTPMetricsExporter(client *http.Client, config *Config) httpMetricsExporter {
	endpoint := config.Metrics.Endpoint
	if isEmpty(endpoint) {
		endpoint = ensureHTTPScheme(config.getDomainGrpcSettings().Endpoint)
	}

	return httpMetricsExporter{
		httpExporter: httpExporter{
			Client:   client,
			Endpoint: endpoint,
		},
	}
}

func newHTTPTracesExporter(client *http.Client, config *Config) httpTracesExporter {
	endpoint := config.Traces.Endpoint
	if isEmpty(endpoint) {
		endpoint = ensureHTTPScheme(config.getDomainGrpcSettings().Endpoint)
	}

	return httpTracesExporter{
		httpExporter: httpExporter{
			Client:   client,
			Endpoint: endpoint,
		},
	}
}

// doRequest performs the HTTP request with the given body and path
func (c *httpExporter) doRequest(ctx context.Context, body []byte, path string) ([]byte, error) {
	url := c.Endpoint
	if path != "" {
		if url[len(url)-1] == '/' {
			url = url[:len(url)-1]
		}
		url += path
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       respBody,
			Message:    fmt.Sprintf("request failed with status code %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	return respBody, nil
}

func (e *httpLogsExporter) Export(ctx context.Context, request plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	body, err := request.MarshalProto()
	response := plogotlp.NewExportResponse()

	if err != nil {
		return response, fmt.Errorf("failed to marshal logs: %w", err)
	}

	resp, err := e.doRequest(ctx, body, "/v1/logs")
	if err != nil {
		return response, err
	}

	if err := response.UnmarshalProto(resp); err != nil {
		return response, fmt.Errorf("failed to unmarshal logs response: %w", err)
	}

	return response, nil
}

func (e *httpMetricsExporter) Export(ctx context.Context, request pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	body, err := request.MarshalProto()
	response := pmetricotlp.NewExportResponse()

	if err != nil {
		return response, fmt.Errorf("failed to marshal metrics: %w", err)
	}

	resp, err := e.doRequest(ctx, body, "/v1/metrics")
	if err != nil {
		return response, err
	}

	if err := response.UnmarshalProto(resp); err != nil {
		return response, fmt.Errorf("failed to unmarshal metrics response: %w", err)
	}

	return response, nil
}

func (e *httpTracesExporter) Export(ctx context.Context, request ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	body, err := request.MarshalProto()
	response := ptraceotlp.NewExportResponse()

	if err != nil {
		return response, fmt.Errorf("failed to marshal traces: %w", err)
	}

	resp, err := e.doRequest(ctx, body, "/v1/traces")
	if err != nil {
		return response, err
	}

	if err := response.UnmarshalProto(resp); err != nil {
		return response, fmt.Errorf("failed to unmarshal traces response: %w", err)
	}

	return response, nil
}
