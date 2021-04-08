// Copyright 2021, OpenTelemetry Authors
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
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
)

// Describes a payload of unstructured events to send to Humio
type HumioUnstructuredEvents struct {
	Fields   map[string]string `json:"fields,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
	Type     string            `json:"type,omitempty"`
	Messages []string          `json:"messages"`
}

// Describes a payload of structured events to send to Humio
type HumioStructuredEvents struct {
	Tags   map[string]string       `json:"tags,omitempty"`
	Events []*HumioStructuredEvent `json:"events"`
}

// Describes a single structured event to send to Humio
// TODO: We can create our own type for "timeinfo" and squash it into the json
// structure. Then a custom marshaler can determine whether to output ISO or
// Unix, as well as to suppress timezone (or combine it with the ISO string?)
// when using ISO format
type HumioStructuredEvent struct {
	TimeStamp  time.Time   `json:"timestamp"`          // TODO: Handle both ISO and Unix
	TimeZone   string      `json:"timezone,omitempty"` // TODO: Does Go have a time zone struct we can use?
	Attributes interface{} `json:"attributes,omitempty"`
	RawString  string      `json:"rawstring,omitempty"`
}

// An HTTP client for sending unstructured and structured events to Humio
type humioClient struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

// Constructs a new HTTP client for sending payloads to Humio
func newHumioClient(config *Config, logger *zap.Logger) (*humioClient, error) {
	client, err := config.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}

	return &humioClient{
		config: config,
		client: client,
		logger: logger,
	}, nil
}

func (h *humioClient) sendUnstructuredEvents(ctx context.Context, evts []*HumioUnstructuredEvents) error {
	return h.sendEvents(ctx, evts, h.config.unstructuredEndpoint)
}

func (h *humioClient) sendStructuredEvents(ctx context.Context, evts []*HumioStructuredEvents) error {
	return h.sendEvents(ctx, evts, h.config.structuredEndpoint)
}

func (h *humioClient) sendEvents(ctx context.Context, evts interface{}, u *url.URL) error {
	body, err := encodeBody(evts)
	if err != nil {
		return consumererror.Permanent(err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		u.String(),
		body,
	)
	if err != nil {
		return consumererror.Permanent(err)
	}

	for h, v := range h.config.Headers {
		req.Header.Set(h, v)
	}

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}
	// Response body needs to both be read to EOF and closed to avoid leaks
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)

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

func encodeBody(body interface{}) (io.Reader, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b), nil
}
