// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// Splunk HEC codes returned on an accepted (HTTP 200) request whose server is approaching capacity.
	// https://docs.splunk.com/Documentation/Splunk/latest/Data/TroubleshootHTTPEventCollector
	splunkCodeApproachingQueueCapacity = 24
	splunkCodeApproachingAckCapacity   = 25
)

type hecWorker interface {
	send(context.Context, buffer, map[string]string) error
}

type defaultHecWorker struct {
	url     *url.URL
	client  *http.Client
	headers map[string]string
	logger  *zap.Logger
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
			hec.logger.Warn("Splunk HEC accepted the data but is approaching capacity; consider reducing the send rate",
				zap.Int("splunk_response_code", splunkResp.Code),
				zap.String("host", hec.url.String()))
		}
	}

	return nil
}

var _ hecWorker = &defaultHecWorker{}
