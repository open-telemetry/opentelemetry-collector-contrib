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

package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/errors"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/buffer"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/flusher"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"go.uber.org/zap"
)

func init() {
	operator.Register("elastic_output", func() operator.Builder { return NewElasticOutputConfig("") })
}

// NewElasticOutputConfig creates a new elastic output config with default values
func NewElasticOutputConfig(operatorID string) *ElasticOutputConfig {
	return &ElasticOutputConfig{
		OutputConfig:  helper.NewOutputConfig(operatorID, "elastic_output"),
		BufferConfig:  buffer.NewConfig(),
		FlusherConfig: flusher.NewConfig(),
	}
}

// ElasticOutputConfig is the configuration of an elasticsearch output operator.
type ElasticOutputConfig struct {
	helper.OutputConfig `yaml:",inline"`
	BufferConfig        buffer.Config  `json:"buffer" yaml:"buffer"`
	FlusherConfig       flusher.Config `json:"flusher" yaml:"flusher"`

	Addresses  []string     `json:"addresses"             yaml:"addresses,flow"`
	Username   string       `json:"username"              yaml:"username"`
	Password   string       `json:"password"              yaml:"password"`
	CloudID    string       `json:"cloud_id"              yaml:"cloud_id"`
	APIKey     string       `json:"api_key"               yaml:"api_key"`
	IndexField *entry.Field `json:"index_field,omitempty" yaml:"index_field,omitempty"`
	IDField    *entry.Field `json:"id_field,omitempty"    yaml:"id_field,omitempty"`
}

// Build will build an elasticsearch output operator.
func (c ElasticOutputConfig) Build(bc operator.BuildContext) ([]operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(bc)
	if err != nil {
		return nil, err
	}

	cfg := elasticsearch.Config{
		Addresses: c.Addresses,
		Username:  c.Username,
		Password:  c.Password,
		CloudID:   c.CloudID,
		APIKey:    c.APIKey,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, errors.NewError(
			"The Elasticsearch client failed to initialize.",
			"Review the underlying error message to troubleshoot the issue.",
			"underlying_error", err.Error(),
		)
	}

	buffer, err := c.BufferConfig.Build(bc, c.ID())
	if err != nil {
		return nil, err
	}

	flusher := c.FlusherConfig.Build(bc.Logger.SugaredLogger)

	ctx, cancel := context.WithCancel(context.Background())

	elasticOutput := &ElasticOutput{
		OutputOperator: outputOperator,
		buffer:         buffer,
		client:         client,
		indexField:     c.IndexField,
		idField:        c.IDField,
		flusher:        flusher,
		ctx:            ctx,
		cancel:         cancel,
	}

	return []operator.Operator{elasticOutput}, nil
}

// ElasticOutput is an operator that sends entries to elasticsearch.
type ElasticOutput struct {
	helper.OutputOperator
	buffer  buffer.Buffer
	flusher *flusher.Flusher

	client     *elasticsearch.Client
	indexField *entry.Field
	idField    *entry.Field

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start signals to the ElasticOutput to begin flushing
func (e *ElasticOutput) Start() error {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.feedFlusher(e.ctx)
	}()

	return nil
}

// Stop tells the ElasticOutput to stop gracefully
func (e *ElasticOutput) Stop() error {
	e.cancel()
	e.wg.Wait()
	e.flusher.Stop()
	return e.buffer.Close()
}

// Process adds an entry to the outputs buffer
func (e *ElasticOutput) Process(ctx context.Context, entry *entry.Entry) error {
	return e.buffer.Add(ctx, entry)
}

// ProcessMulti will send entries to elasticsearch.
func (e *ElasticOutput) createRequest(entries []*entry.Entry) *esapi.BulkRequest {
	type indexDirective struct {
		Index struct {
			Index string `json:"_index"`
			ID    string `json:"_id"`
		} `json:"index"`
	}

	// The bulk API expects newline-delimited json strings, with an operation directive
	// immediately followed by the document.
	// https://www.elastic.co/guide/en/elasticsearch/reference/master/docs-bulk.html
	var buffer bytes.Buffer
	var err error
	for _, entry := range entries {
		directive := indexDirective{}
		directive.Index.Index, err = e.FindIndex(entry)
		if err != nil {
			e.Warnw("Failed to find index", zap.Any("error", err))
			continue
		}

		directive.Index.ID, err = e.FindID(entry)
		if err != nil {
			e.Warnw("Failed to find id", zap.Any("error", err))
			continue
		}

		directiveJSON, err := json.Marshal(directive)
		if err != nil {
			e.Warnw("Failed to marshal directive JSON", zap.Any("error", err))
			continue
		}

		entryJSON, err := json.Marshal(entry)
		if err != nil {
			e.Warnw("Failed to marshal entry JSON", zap.Any("error", err))
			continue
		}

		buffer.Write(directiveJSON)
		buffer.Write([]byte("\n"))
		buffer.Write(entryJSON)
		buffer.Write([]byte("\n"))
	}

	request := &esapi.BulkRequest{
		Body: bytes.NewReader(buffer.Bytes()),
	}

	return request
}

func (e *ElasticOutput) feedFlusher(ctx context.Context) {
	for {
		entries, clearer, err := e.buffer.ReadChunk(ctx)
		if err != nil && err == context.Canceled {
			return
		} else if err != nil {
			e.Errorf("Failed to read chunk", zap.Error(err))
			continue
		}

		e.flusher.Do(func(ctx context.Context) error {
			req := e.createRequest(entries)
			res, err := req.Do(ctx, e.client)
			if err != nil {
				return errors.NewError(
					"Client failed to submit request to elasticsearch.",
					"Review the underlying error message to troubleshoot the issue",
					"underlying_error", err.Error(),
				)
			}

			if res.IsError() {
				return errors.NewError(
					"Request to elasticsearch returned a failure code.",
					"Review status and status code for further details.",
					"status_code", strconv.Itoa(res.StatusCode),
					"status", res.Status(),
				)
			}

			if err = clearer.MarkAllAsFlushed(); err != nil {
				e.Errorw("Failed to mark entries as flushed", zap.Error(err))
			}
			return nil
		})
	}
}

// FindIndex will find an index that will represent an entry in elasticsearch.
func (e *ElasticOutput) FindIndex(entry *entry.Entry) (string, error) {
	if e.indexField == nil {
		return "default", nil
	}

	var value string
	err := entry.Read(*e.indexField, &value)
	if err != nil {
		return "", errors.Wrap(err, "extract index from record")
	}

	return value, nil
}

// FindID will find the id that will represent an entry in elasticsearch.
func (e *ElasticOutput) FindID(entry *entry.Entry) (string, error) {
	if e.idField == nil {
		return uuid.GenerateUUID()
	}

	var value string
	err := entry.Read(*e.idField, &value)
	if err != nil {
		return "", errors.Wrap(err, "extract id from record")
	}

	return value, nil
}
