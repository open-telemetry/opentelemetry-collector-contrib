// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var defaultRoundTripFunc = func(*http.Request) (*http.Response, error) {
	return &http.Response{
		Body: io.NopCloser(strings.NewReader("{}")),
	}, nil
}

type mockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.RoundTripFunc == nil {
		return defaultRoundTripFunc(req)
	}
	return t.RoundTripFunc(req)
}

const successResp = `{
  "took": 30,
  "errors": false,
  "items": [
    {
      "create": {
        "_index": "foo",
        "status": 201
      }
    }
  ]
}`

func TestBulkIndexer_addBatchAndFlush(t *testing.T) {
	cfg := Config{NumWorkers: 1}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
		RoundTripFunc: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
				Body:   io.NopCloser(strings.NewReader(successResp)),
			}, nil
		},
	}})
	require.NoError(t, err)
	bulkIndexer, err := newBulkIndexer(zap.NewNop(), client, &cfg)
	require.NoError(t, err)
	assert.NoError(t, bulkIndexer.AddBatchAndFlush(context.Background(),
		[]esBulkIndexerItem{
			{
				Index: "foo",
				Body:  strings.NewReader(`{"foo": "bar"}`),
			},
		}))
	assert.Equal(t, int64(1), bulkIndexer.stats.docsIndexed.Load())
	assert.NoError(t, bulkIndexer.Close(context.Background()))
}

func TestBulkIndexer_addBatchAndFlush_error(t *testing.T) {
	tests := []struct {
		name          string
		roundTripFunc func(*http.Request) (*http.Response, error)
	}{
		{
			name: "500",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 500,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader("error")),
				}, nil
			},
		},
		{
			name: "429",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 429,
					Header:     http.Header{"X-Elastic-Product": []string{"Elasticsearch"}},
					Body:       io.NopCloser(strings.NewReader("error")),
				}, nil
			},
		},
		{
			name: "transport error",
			roundTripFunc: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport error")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{NumWorkers: 1}
			client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: &mockTransport{
				RoundTripFunc: tt.roundTripFunc,
			}})
			require.NoError(t, err)
			bulkIndexer, err := newBulkIndexer(zap.NewNop(), client, &cfg)
			require.NoError(t, err)
			assert.ErrorContains(t, bulkIndexer.AddBatchAndFlush(context.Background(),
				[]esBulkIndexerItem{
					{
						Index: "foo",
						Body:  strings.NewReader(`{"foo": "bar"}`),
					},
				}), "failed to execute the request")
			assert.Equal(t, int64(0), bulkIndexer.stats.docsIndexed.Load())
			assert.NoError(t, bulkIndexer.Close(context.Background()))
		})
	}
}
