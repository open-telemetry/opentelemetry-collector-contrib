// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmexporter

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/klauspost/compress/zstd"
	splunksapm "github.com/signalfx/sapm-proto/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func TestCreateTraces(t *testing.T) {
	cfg := &Config{
		Endpoint:           "test-endpoint",
		AccessToken:        "abcd1234",
		NumWorkers:         3,
		MaxConnections:     45,
		DisableCompression: true,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
	}
	params := exportertest.NewNopSettings(metadata.Type)

	te, err := newSAPMTracesExporter(cfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	assert.NoError(t, te.Shutdown(context.Background()), "trace exporter shutdown failed")
}

func buildTestTraces(setTokenLabel bool) (traces ptrace.Traces) {
	traces = ptrace.NewTraces()
	rss := traces.ResourceSpans()
	rss.EnsureCapacity(20)

	for i := 0; i < 20; i++ {
		rs := rss.AppendEmpty()
		resource := rs.Resource()
		resource.Attributes().PutStr("key1", "value1")
		if setTokenLabel && i%2 == 1 {
			tokenLabel := fmt.Sprintf("MyToken%d", i/5)
			resource.Attributes().PutStr("com.splunk.signalfx.access_token", tokenLabel)
			resource.Attributes().PutStr("com.splunk.signalfx.access_token", tokenLabel)
		}
		// Add one last element every 3rd resource, this way we have cases with token last or not.
		if i%3 == 1 {
			resource.Attributes().PutStr("key2", "value2")
		}

		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		name := fmt.Sprintf("Span%d", i)
		span.SetName(name)
		span.SetTraceID([16]byte{1})
		span.SetSpanID([8]byte{1})
	}

	return traces
}

func TestFilterToken(t *testing.T) {
	tests := []struct {
		name     string
		useToken bool
	}{
		{
			name:     "no token",
			useToken: false,
		},
		{
			name:     "some with token",
			useToken: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := buildTestTraces(tt.useToken)
			batches := jaeger.ProtoFromTraces(traces)
			assert.Equal(t, tt.useToken, hasToken(batches))
			filterToken(batches)
			assert.False(t, hasToken(batches))
		})
	}
}

func hasToken(batches []*model.Batch) bool {
	for _, batch := range batches {
		proc := batch.Process
		if proc == nil {
			continue
		}
		for i := range proc.Tags {
			if proc.Tags[i].Key == splunk.SFxAccessTokenLabel {
				return true
			}
		}
	}
	return false
}

func buildTestTrace() (ptrace.Traces, error) {
	trace := ptrace.NewTraces()
	trace.ResourceSpans().EnsureCapacity(2)
	for i := 0; i < 2; i++ {
		rs := trace.ResourceSpans().AppendEmpty()
		resource := rs.Resource()
		resource.Attributes().PutStr("com.splunk.signalfx.access_token", fmt.Sprintf("TraceAccessToken%v", i))
		span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("MySpan")

		var traceIDBytes [16]byte
		var spanIDBytes [8]byte
		_, err := rand.Read(traceIDBytes[:])
		if err != nil {
			return trace, err
		}
		_, err = rand.Read(spanIDBytes[:])
		if err != nil {
			return trace, err
		}
		span.SetTraceID(traceIDBytes)
		span.SetSpanID(spanIDBytes)
	}
	return trace, nil
}

func TestSAPMClientTokenUsageAndErrorMarshalling(t *testing.T) {
	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		sendError              bool
	}{
		{
			name:                   "no error without passthrough",
			accessTokenPassthrough: false,
			sendError:              false,
		},
		{
			name:                   "no error with passthrough",
			accessTokenPassthrough: true,
			sendError:              false,
		},
		{
			name:                   "error without passthrough",
			accessTokenPassthrough: false,
			sendError:              true,
		},
		{
			name:                   "error with passthrough",
			accessTokenPassthrough: true,
			sendError:              true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracesReceived := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedToken := "ClientAccessToken"
				if tt.accessTokenPassthrough {
					expectedToken = "TraceAccessToken"
				}
				assert.Contains(t, r.Header.Get("x-sf-token"), expectedToken)
				status := 200
				if tt.sendError {
					status = 400
				}
				w.WriteHeader(status)
				tracesReceived = true
			}))
			defer func() {
				assert.True(t, tracesReceived, "Test server never received traces.")
			}()
			defer server.Close()

			cfg := &Config{
				Endpoint:    server.URL,
				AccessToken: "ClientAccessToken",
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: tt.accessTokenPassthrough,
				},
			}
			params := exportertest.NewNopSettings(metadata.Type)

			se, err := newSAPMExporter(cfg, params)
			assert.NoError(t, err)
			assert.NotNil(t, se, "failed to create trace exporter")

			trace, testTraceErr := buildTestTrace()
			require.NoError(t, testTraceErr)
			err = se.pushTraceData(context.Background(), trace)

			if tt.sendError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSAPMClientTokenAccess(t *testing.T) {
	tests := []struct {
		name                   string
		inContext              bool
		accessTokenPassthrough bool
	}{
		{
			name:                   "Token in context with passthrough",
			inContext:              true,
			accessTokenPassthrough: true,
		},
		{
			name:                   "Token in attributes with passthrough",
			inContext:              false,
			accessTokenPassthrough: true,
		},
		{
			name:                   "Token in config without passthrough",
			inContext:              false,
			accessTokenPassthrough: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracesReceived := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedToken := "ClientAccessToken"
				if tt.accessTokenPassthrough && tt.inContext {
					expectedToken = "SplunkAccessToken"
				} else if tt.accessTokenPassthrough && !tt.inContext {
					expectedToken = "TraceAccessToken0"
				}
				assert.Contains(t, r.Header.Get("x-sf-token"), expectedToken)
				status := 200
				w.WriteHeader(status)
				tracesReceived = true
			}))
			defer func() {
				assert.True(t, tracesReceived, "Test server never received traces.")
			}()
			defer server.Close()

			cfg := &Config{
				Endpoint:    server.URL,
				AccessToken: "ClientAccessToken",
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: tt.accessTokenPassthrough,
				},
			}
			params := exportertest.NewNopSettings(metadata.Type)

			se, err := newSAPMExporter(cfg, params)
			assert.NoError(t, err)
			assert.NotNil(t, se, "failed to create trace exporter")

			trace, testTraceErr := buildTestTrace()
			require.NoError(t, testTraceErr)

			ctx := context.Background()
			if tt.inContext {
				ctx = client.NewContext(
					ctx,
					client.Info{Metadata: client.NewMetadata(
						map[string][]string{splunk.SFxAccessTokenHeader: {"SplunkAccessToken"}},
					)},
				)
			}
			err = se.pushTraceData(ctx, trace)
			require.NoError(t, err)
		})
	}
}

func decompress(body io.Reader, compression string) ([]byte, error) {
	switch compression {
	case "":
		return io.ReadAll(body)
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	case "zstd":
		reader, err := zstd.NewReader(body)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("unknown compression %q", compression)
}

func TestCompression(t *testing.T) {
	tests := []struct {
		name                     string
		configDisableCompression bool
		configCompression        string
		receivedCompression      string
	}{
		{
			name:                     "unspecified config",
			configCompression:        "",
			configDisableCompression: false,
			receivedCompression:      "gzip",
		},
		{
			name:                     "gzip",
			configCompression:        "gzip",
			configDisableCompression: false,
			receivedCompression:      "gzip",
		},
		{
			name:                     "zstd",
			configCompression:        "zstd",
			configDisableCompression: false,
			receivedCompression:      "zstd",
		},
		{
			name:                     "disable compression and unspecified method",
			configDisableCompression: true,
			configCompression:        "",
			receivedCompression:      "",
		},
		{
			name:                     "disable compression and specify gzip",
			configDisableCompression: true,
			configCompression:        "gzip",
			receivedCompression:      "",
		},
		{
			name:                     "disable compression and specify zstd",
			configDisableCompression: true,
			configCompression:        "zstd",
			receivedCompression:      "",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				tracesReceived := false
				server := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							compression := r.Header.Get("Content-Encoding")
							assert.Equal(t, compression, tt.receivedCompression)

							payload, err := decompress(r.Body, compression)
							assert.NoError(t, err)

							var sapm splunksapm.PostSpansRequest
							err = sapm.Unmarshal(payload)
							assert.NoError(t, err)

							w.WriteHeader(http.StatusOK)
							tracesReceived = true
						},
					),
				)
				defer func() {
					assert.True(t, tracesReceived, "Test server never received traces.")
				}()
				defer server.Close()

				cfg := &Config{
					Endpoint:           server.URL,
					DisableCompression: tt.configDisableCompression,
					Compression:        tt.configCompression,
				}
				params := exportertest.NewNopSettings(metadata.Type)

				se, err := newSAPMExporter(cfg, params)
				assert.NoError(t, err)
				assert.NotNil(t, se, "failed to create trace exporter")

				trace, testTraceErr := buildTestTrace()
				require.NoError(t, testTraceErr)
				err = se.pushTraceData(context.Background(), trace)
				require.NoError(t, err)
			},
		)
	}
}
