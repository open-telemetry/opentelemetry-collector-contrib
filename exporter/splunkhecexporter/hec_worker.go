// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type hecWorker interface {
	send(context.Context, buffer, map[string]string, int) error
}

type httpclient interface {
	Do(req *http.Request) (*http.Response, error)
}
type defaultHecWorker struct {
	url            *url.URL
	client         httpclient
	headers        map[string]string
	logger         *zap.Logger
	contentLimit   *int
	eventBytesSent atomic.Int64
}

func (hec *defaultHecWorker) send(ctx context.Context, buf buffer, headers map[string]string, payloadSize int) error {
	req, err := http.NewRequestWithContext(ctx, "POST", hec.url.String(), buf)
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
		hec.eventBytesSent.Store(0)
		return err
	}

	// Do not drain the response when 429 or 502 status code is returned.
	// HTTP client will not reuse the same connection unless it is drained.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18281 for more details.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadGateway {
		hec.eventBytesSent.Store(0)
		return nil
	}

	if hec.contentLimit != nil {
		newSent := hec.eventBytesSent.Add(int64(payloadSize))
		// We have sent past the limit of payload bytes we mean to send
		// and are going to explicitly reconnect to the backend
		// so that this connection can be balanced across multiple
		// indexers.
		if int64(*hec.contentLimit) <= newSent {
			hec.eventBytesSent.Store(0)
			return nil
		}
	}

	_, errCopy := io.Copy(io.Discard, resp.Body)
	return errCopy
}

var _ hecWorker = &defaultHecWorker{}
