// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// Splunk HEC codes returned on an accepted (HTTP 200) request whose server is approaching capacity.
	// https://docs.splunk.com/Documentation/Splunk/latest/Data/TroubleshootHTTPEventCollector
	splunkCodeApproachingQueueCapacity = 24
	splunkCodeApproachingAckCapacity   = 25
)

// errPayloadTooLarge marks a batch the server rejected as too large (HTTP 413).
// Wrapped permanent so non-splitting signals drop it; the logs path detects it
// (errors.Is) and splits instead.
var errPayloadTooLarge = errors.New("splunk hec payload too large")

type hecWorker interface {
	send(context.Context, buffer, map[string]string) error
}

type defaultHecWorker struct {
	url     *url.URL
	client  *http.Client
	headers map[string]string
	logger  *zap.Logger
	// telemetry is nil when the exporter has not started, e.g. in tests.
	telemetry *metadata.TelemetryBuilder
}

func (hec *defaultHecWorker) send(ctx context.Context, buf buffer, headers map[string]string) error {
	// We copy the bytes to a new buffer to avoid corruption. This is a workaround to avoid hitting https://github.com/golang/go/issues/51907.
	nb := make([]byte, buf.Len())
	copy(nb, buf.Bytes())
	bodyBuf := bytes.NewReader(nb)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hec.url.String(), bodyBuf)
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.ContentLength = int64(buf.Len())

	// Set the headers configured for the client
	for k, v := range hec.headers {
		req.Header.Set(k, v)
	}

	// Set extra headers passed by the caller
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if _, ok := buf.(*cancellableGzipWriter); ok {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := hec.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		hec.logger.Error("Splunk is unable to receive data. Please investigate the health of the cluster", zap.Int("status", resp.StatusCode), zap.String("host", hec.url.String()))
	}

	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		_, _ = io.Copy(io.Discard, resp.Body)
		return consumererror.NewPermanent(fmt.Errorf("%w: HTTP %q %d %q", errPayloadTooLarge,
			resp.Request.URL.Path, resp.StatusCode, http.StatusText(resp.StatusCode)))
	}

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	// Data was accepted (200), so codes 24/25 warn rather than retry (a retry would duplicate it).
	// Read enough to find the code, then drain the rest so the connection can be reused.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	_, _ = io.Copy(io.Discard, resp.Body)
	var splunkResp struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(body, &splunkResp) == nil {
		switch splunkResp.Code {
		case splunkCodeApproachingQueueCapacity, splunkCodeApproachingAckCapacity:
			if hec.telemetry != nil {
				hec.telemetry.ExporterSplunkhecApproachingCapacity.Add(ctx, 1,
					metric.WithAttributes(attribute.Int("splunk_response_code", splunkResp.Code)))
			}
			hec.logger.Debug("Splunk HEC accepted the data and reported it is approaching capacity",
				zap.Int("splunk_response_code", splunkResp.Code),
				zap.String("host", hec.url.String()))
		}
	}

	return nil
}

var _ hecWorker = &defaultHecWorker{}
