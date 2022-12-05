package splunkhecexporter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/multierr"
)

type hecWorker interface {
	Send(context.Context, *request) error
}

type hecWorkerWithoutAck struct {
	url     *url.URL
	headers map[string]string
	client  *http.Client
}

func newHecWorkerWithoutAck(config *Config, clientFunc func() *http.Client) *hecWorkerWithoutAck {
	url, _ := config.getURL()
	return &hecWorkerWithoutAck{
		client:  clientFunc(),
		headers: buildHttpHeaders(config),
		url:     url,
	}
}

func (hec *hecWorkerWithoutAck) Send(ctx context.Context, request *request) error {
	req, err := http.NewRequestWithContext(ctx, "POST", hec.url.String(), bytes.NewBuffer(request.bytes))
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	req.ContentLength = int64(len(request.bytes))

	// Set the headers configured for the client
	for k, v := range hec.headers {
		req.Header.Set(k, v)
	}

	// Set extra headers passed by the caller
	for k, v := range request.localHeaders {
		req.Header.Set(k, v)
	}

	if request.compressionEnabled {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := hec.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	_, errCopy := io.Copy(io.Discard, resp.Body)
	return multierr.Combine(err, errCopy)
}

var _ hecWorker = &hecWorkerWithoutAck{}
