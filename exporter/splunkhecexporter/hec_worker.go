// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
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

	// Do not drain the response when 429 or 502 status code is returned.
	// HTTP client will not reuse the same connection unless it is drained.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18281 for more details.
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusBadGateway {
		if _, errCopy := io.Copy(io.Discard, resp.Body); errCopy != nil {
			return errCopy
		}
	}
	return nil
}

var _ hecWorker = &defaultHecWorker{}
