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

package newrelic

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/errors"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/buffer"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/flusher"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"go.uber.org/zap"
)

func init() {
	operator.Register("newrelic_output", func() operator.Builder { return NewNewRelicOutputConfig("") })
}

// NewNewRelicOutputConfig creates a new relic output config with default values
func NewNewRelicOutputConfig(operatorID string) *NewRelicOutputConfig {
	return &NewRelicOutputConfig{
		OutputConfig:  helper.NewOutputConfig(operatorID, "newrelic_output"),
		BufferConfig:  buffer.NewConfig(),
		FlusherConfig: flusher.NewConfig(),
		BaseURI:       "https://log-api.newrelic.com/log/v1",
		Timeout:       helper.NewDuration(10 * time.Second),
		MessageField:  entry.NewRecordField(),
	}
}

// NewRelicOutputConfig is the configuration of a NewRelicOutput operator
type NewRelicOutputConfig struct {
	helper.OutputConfig `yaml:",inline"`
	BufferConfig        buffer.Config  `json:"buffer" yaml:"buffer"`
	FlusherConfig       flusher.Config `json:"flusher" yaml:"flusher"`

	APIKey       string          `json:"api_key,omitempty"       yaml:"api_key,omitempty"`
	BaseURI      string          `json:"base_uri,omitempty"      yaml:"base_uri,omitempty"`
	LicenseKey   string          `json:"license_key,omitempty"   yaml:"license_key,omitempty"`
	Timeout      helper.Duration `json:"timeout,omitempty"       yaml:"timeout,omitempty"`
	MessageField entry.Field     `json:"message_field,omitempty" yaml:"message_field,omitempty"`
}

// Build will build a new NewRelicOutput
func (c NewRelicOutputConfig) Build(bc operator.BuildContext) ([]operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(bc)
	if err != nil {
		return nil, err
	}

	headers, err := c.getHeaders()
	if err != nil {
		return nil, err
	}

	buffer, err := c.BufferConfig.Build(bc, c.ID())
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(c.BaseURI)
	if err != nil {
		return nil, errors.Wrap(err, "'base_uri' is not a valid URL")
	}

	flusher := c.FlusherConfig.Build(bc.Logger.SugaredLogger)
	ctx, cancel := context.WithCancel(context.Background())

	nro := &NewRelicOutput{
		OutputOperator: outputOperator,
		buffer:         buffer,
		flusher:        flusher,
		client:         &http.Client{},
		headers:        headers,
		url:            url,
		timeout:        c.Timeout.Raw(),
		messageField:   c.MessageField,
		ctx:            ctx,
		cancel:         cancel,
	}

	return []operator.Operator{nro}, nil
}

func (c NewRelicOutputConfig) getHeaders() (http.Header, error) {
	headers := http.Header{
		"X-Event-Source":   []string{"logs"},
		"Content-Encoding": []string{"gzip"},
	}

	if c.APIKey == "" && c.LicenseKey == "" {
		return nil, fmt.Errorf("one of 'api_key' or 'license_key' is required")
	} else if c.APIKey != "" {
		headers["X-Insert-Key"] = []string{c.APIKey}
	} else {
		headers["X-License-Key"] = []string{c.LicenseKey}
	}

	return headers, nil
}

// NewRelicOutput is an operator that sends entries to the New Relic Logs platform
type NewRelicOutput struct {
	helper.OutputOperator
	buffer  buffer.Buffer
	flusher *flusher.Flusher

	client       *http.Client
	url          *url.URL
	headers      http.Header
	timeout      time.Duration
	messageField entry.Field

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start tests the connection to New Relic and begins flushing entries
func (nro *NewRelicOutput) Start() error {
	if err := nro.testConnection(); err != nil {
		return fmt.Errorf("test connection: %s", err)
	}

	nro.wg.Add(1)
	go func() {
		defer nro.wg.Done()
		nro.feedFlusher(nro.ctx)
	}()

	return nil
}

// Stop tells the NewRelicOutput to stop gracefully
func (nro *NewRelicOutput) Stop() error {
	nro.cancel()
	nro.wg.Wait()
	nro.flusher.Stop()
	return nro.buffer.Close()
}

// Process adds an entry to the output's buffer
func (nro *NewRelicOutput) Process(ctx context.Context, entry *entry.Entry) error {
	return nro.buffer.Add(ctx, entry)
}

func (nro *NewRelicOutput) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), nro.timeout)
	defer cancel()

	req, err := nro.newRequest(ctx, nil)
	if err != nil {
		return err
	}

	res, err := nro.client.Do(req)
	if err != nil {
		return err
	}

	return nro.handleResponse(res)
}

func (nro *NewRelicOutput) feedFlusher(ctx context.Context) {
	for {
		entries, clearer, err := nro.buffer.ReadChunk(ctx)
		if err != nil && err == context.Canceled {
			return
		} else if err != nil {
			nro.Errorf("Failed to read chunk", zap.Error(err))
			continue
		}

		nro.flusher.Do(func(ctx context.Context) error {
			req, err := nro.newRequest(ctx, entries)
			if err != nil {
				nro.Errorw("Failed to create request from payload", zap.Error(err))
				// drop these logs because we couldn't creat a request and a retry won't help
				if err := clearer.MarkAllAsFlushed(); err != nil {
					nro.Errorf("Failed to mark entries as flushed after failing to create a request", zap.Error(err))
				}
				return nil
			}

			res, err := nro.client.Do(req)
			if err != nil {
				return err
			}

			if err := nro.handleResponse(res); err != nil {
				return err
			}

			if err = clearer.MarkAllAsFlushed(); err != nil {
				nro.Errorw("Failed to mark entries as flushed", zap.Error(err))
			}
			return nil
		})
	}
}

// newRequest creates a new http.Request with the given context and entries
func (nro *NewRelicOutput) newRequest(ctx context.Context, entries []*entry.Entry) (*http.Request, error) {
	payload := LogPayloadFromEntries(entries, nro.messageField)

	var buf bytes.Buffer
	wr := gzip.NewWriter(&buf)
	enc := json.NewEncoder(wr)
	if err := enc.Encode(payload); err != nil {
		return nil, errors.Wrap(err, "encode payload")
	}
	if err := wr.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", nro.url.String(), &buf)
	if err != nil {
		return nil, err
	}
	req.Header = nro.headers

	return req, nil
}

func (nro *NewRelicOutput) handleResponse(res *http.Response) error {
	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.NewError("unexpected status code", "", "status", res.Status)
		} else {
			res.Body.Close()
			return errors.NewError("unexpected status code", "", "status", res.Status, "body", string(body))
		}
	}
	res.Body.Close()
	return nil
}
