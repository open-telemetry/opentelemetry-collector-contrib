// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package humioexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
)

// HumioUnstructuredEvents represents a payload of multiple unstructured events (strings) to send to Humio
type HumioUnstructuredEvents struct {
	// Key-value pairs to associate with the messages as metadata
	Fields map[string]string `json:"fields,omitempty"`

	// Tags used to target specific data sources in Humio
	Tags map[string]string `json:"tags,omitempty"`

	// The name of the parser to handle these messages inside Humio
	Type string `json:"type,omitempty"`

	// The series of unstructured messages
	Messages []string `json:"messages"`
}

// HumioStructuredEvents represents a payload of multiple structured events to send to Humio
type HumioStructuredEvents struct {
	// Tags used to target specific data sources in Humio
	Tags map[string]string `json:"tags,omitempty"`

	// The series of structured events
	Events []*HumioStructuredEvent `json:"events"`
}

// HumioStructuredEvent represents a single structured event to send to Humio
type HumioStructuredEvent struct {
	// The time where the event occurred
	Timestamp time.Time

	// Whether to serialize the timestamp as Unix or ISO
	AsUnix bool

	// The event payload
	Attributes interface{}
}

// MarshalJSON formats the timestamp in a HumioStructuredEvent as either an ISO string or a
// Unix timestamp in milliseconds with time zone
func (e *HumioStructuredEvent) MarshalJSON() ([]byte, error) {
	if e.AsUnix {
		return json.Marshal(struct {
			Timestamp  int64       `json:"timestamp"`
			TimeZone   string      `json:"timezone"`
			Attributes interface{} `json:"attributes,omitempty"`
		}{
			Timestamp:  e.Timestamp.Local().UnixNano() * int64(time.Nanosecond) / int64(time.Millisecond),
			TimeZone:   e.Timestamp.Location().String(),
			Attributes: e.Attributes,
		})
	}

	return json.Marshal(struct {
		Timestamp  time.Time   `json:"timestamp"`
		Attributes interface{} `json:"attributes,omitempty"`
	}{
		Timestamp:  e.Timestamp,
		Attributes: e.Attributes,
	})
}

// Abstract interface describing the capabilities of an HTTP client for sending
// unstructured and structured events
type exporterClient interface {
	sendUnstructuredEvents(context.Context, []*HumioUnstructuredEvents) error
	sendStructuredEvents(context.Context, []*HumioStructuredEvents) error
}

// A concrete HTTP client for sending unstructured and structured events to Humio
type humioClient struct {
	cfg      *Config
	client   *http.Client
	gzipPool *sync.Pool
	logger   *zap.Logger
}

// Constructs a new HTTP client for sending payloads to Humio
func newHumioClient(cfg *Config, logger *zap.Logger, host component.Host) (exporterClient, error) {
	client, err := cfg.HTTPClientSettings.ToClient(host.GetExtensions())
	if err != nil {
		return nil, err
	}

	return &humioClient{
		cfg:    cfg,
		client: client,
		gzipPool: &sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		logger: logger,
	}, nil
}

// Send a payload of unstructured events to the corresponding Humio API. Will eventually be renamed as sendLogs
func (h *humioClient) sendUnstructuredEvents(ctx context.Context, evts []*HumioUnstructuredEvents) error {
	return h.sendEvents(ctx, evts, h.cfg.unstructuredEndpoint.String(), h.cfg.Logs.IngestToken)
}

// Send a payload of structured events to the corresponding Humio API. Will eventually be renamed as sendTraces
func (h *humioClient) sendStructuredEvents(ctx context.Context, evts []*HumioStructuredEvents) error {
	return h.sendEvents(ctx, evts, h.cfg.structuredEndpoint.String(), h.cfg.Traces.IngestToken)
}

// Send a payload of generic events to the specified Humio API. This method should
// never be called directly
func (h *humioClient) sendEvents(ctx context.Context, evts interface{}, url string, token string) error {
	body, err := h.encodeBody(evts)
	if err != nil {
		return consumererror.Permanent(err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		url,
		body,
	)
	if err != nil {
		return consumererror.Permanent(err)
	}

	for h, v := range h.cfg.Headers {
		req.Header.Set(h, v)
	}
	req.Header.Set("authorization", "Bearer "+token)

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	// Response body needs to both be read to EOF and closed to avoid leaks
	defer res.Body.Close()
	io.Copy(ioutil.Discard, res.Body)

	// If an error has occurred, determine if it would make sense to retry
	// This check is not exhaustive, but should cover the most common cases
	if res.StatusCode < http.StatusOK ||
		res.StatusCode >= http.StatusMultipleChoices {
		err = errors.New("unable to export events to Humio, got " + res.Status)

		// These indicate a programming or configuration error
		if res.StatusCode == http.StatusBadRequest ||
			res.StatusCode == http.StatusUnauthorized ||
			res.StatusCode == http.StatusForbidden {
			return consumererror.Permanent(err)
		}

		return err
	}

	return nil
}

// Encode the specified payload as json
func (h *humioClient) encodeBody(body interface{}) (io.Reader, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	if h.cfg.DisableCompression {
		return bytes.NewReader(b), nil
	}
	return h.compressBody(b)
}

func (h *humioClient) compressBody(body []byte) (io.Reader, error) {
	gzipper := h.gzipPool.Get().(*gzip.Writer)
	defer h.gzipPool.Put(gzipper)

	// Must reset writer because we reuse it
	b := new(bytes.Buffer)
	gzipper.Reset(b)

	_, err := gzipper.Write(body)
	if err != nil {
		return nil, err
	}

	err = gzipper.Close()
	if err != nil {
		return nil, err
	}

	return b, nil
}
