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

package sapmexporter

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func TestCreateTraceExporter(t *testing.T) {
	config := &Config{
		ExporterSettings:   configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "sapm/customname"},
		Endpoint:           "test-endpoint",
		AccessToken:        "abcd1234",
		NumWorkers:         3,
		MaxConnections:     45,
		DisableCompression: true,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newSAPMTraceExporter(config, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	assert.NoError(t, te.Shutdown(context.Background()), "trace exporter shutdown failed")
}

func TestCreateTraceExporterWithCorrelationEnabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1234"
	cfg.Correlation.Enabled = true
	cfg.Correlation.Endpoint = "http://localhost"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newSAPMExporter(cfg, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	assert.NotNil(t, te.tracker, "correlation tracker should have been set")
}

func TestCreateTraceExporterWithCorrelationDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1234"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newSAPMExporter(cfg, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	assert.Nil(t, te.tracker, "tracker correlation should not be created")
}

func TestCreateTraceExporterWithInvalidConfig(t *testing.T) {
	config := &Config{}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	te, err := newSAPMTraceExporter(config, params)
	require.Error(t, err)
	assert.Nil(t, te)
}

func buildTestTraces(setTokenLabel, accessTokenPassthrough bool) (traces pdata.Traces, expected map[string]pdata.Traces) {
	traces = pdata.NewTraces()
	expected = map[string]pdata.Traces{}
	rss := traces.ResourceSpans()
	rss.Resize(50)

	for i := 0; i < 50; i++ {
		span := rss.At(i)
		span.InitEmpty()
		resource := span.Resource()
		resource.InitEmpty()
		token := ""
		if setTokenLabel && i%2 == 1 {
			tokenLabel := fmt.Sprintf("MyToken%d", i/25)
			resource.Attributes().InsertString("com.splunk.signalfx.access_token", tokenLabel)
			if accessTokenPassthrough {
				token = tokenLabel
			}
		}

		resource.Attributes().InsertString("MyLabel", "MyLabelValue")
		span.InstrumentationLibrarySpans().Resize(1)
		span.InstrumentationLibrarySpans().At(0).Spans().Resize(1)
		name := fmt.Sprintf("Span%d", i)
		span.InstrumentationLibrarySpans().At(0).Spans().At(0).SetName(name)

		trace, contains := expected[token]
		if !contains {
			trace = pdata.NewTraces()
			expected[token] = trace
		}
		newSize := trace.ResourceSpans().Len() + 1
		trace.ResourceSpans().Resize(newSize)
		span.CopyTo(trace.ResourceSpans().At(newSize - 1))
		trace.ResourceSpans().At(newSize - 1).Resource().Attributes().Delete("com.splunk.signalfx.access_token")
	}

	return traces, expected
}

func assertTracesEqual(t *testing.T, expected, actual map[string]pdata.Traces) {
	for token, trace := range expected {
		aTrace := actual[token]
		eResourceSpans := trace.ResourceSpans()
		aResourceSpans := aTrace.ResourceSpans()
		assert.Equal(t, eResourceSpans.Len(), aResourceSpans.Len())
		for i := 0; i < eResourceSpans.Len(); i++ {
			eAttrs := eResourceSpans.At(i).Resource().Attributes()
			aAttrs := aResourceSpans.At(i).Resource().Attributes()
			assert.Equal(t, eAttrs.Len(), aAttrs.Len())
			aAttrs.ForEach(func(k string, v pdata.AttributeValue) {
				eVal, _ := eAttrs.Get(k)
				assert.EqualValues(t, eVal, v)
			})

			eSpans := eResourceSpans.At(i).InstrumentationLibrarySpans()
			aSpans := aResourceSpans.At(i).InstrumentationLibrarySpans()
			assert.Equal(t, eSpans.Len(), aSpans.Len())
		}
	}
}

func TestAccessTokenPassthrough(t *testing.T) {
	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		useToken               bool
	}{
		{
			name:                   "no token without passthrough",
			accessTokenPassthrough: false,
			useToken:               false,
		},
		{
			name:                   "no token with passthrough",
			accessTokenPassthrough: true,
			useToken:               false,
		},
		{
			name:                   "token without passthrough",
			accessTokenPassthrough: false,
			useToken:               true,
		},
		{
			name:                   "token with passthrough",
			accessTokenPassthrough: true,
			useToken:               true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Endpoint: "localhost",
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: tt.accessTokenPassthrough,
				},
			}
			params := component.ExporterCreateParams{Logger: zap.NewNop()}

			se, err := newSAPMExporter(config, params)
			assert.Nil(t, err)
			assert.NotNil(t, se, "failed to create trace exporter")

			traces, expected := buildTestTraces(tt.useToken, tt.accessTokenPassthrough)

			actual := se.tracesByAccessToken(traces)
			assertTracesEqual(t, expected, actual)
		})
	}
}

func buildTestTrace(setIds bool) pdata.Traces {
	trace := pdata.NewTraces()
	trace.ResourceSpans().Resize(2)
	for i := 0; i < 2; i++ {
		span := trace.ResourceSpans().At(i)
		span.InitEmpty()
		resource := span.Resource()
		resource.InitEmpty()
		resource.Attributes().InsertString("com.splunk.signalfx.access_token", fmt.Sprintf("TraceAccessToken%v", i))
		span.InstrumentationLibrarySpans().Resize(1)
		span.InstrumentationLibrarySpans().At(0).Spans().Resize(1)
		span.InstrumentationLibrarySpans().At(0).Spans().At(0).SetName("MySpan")

		rand.Seed(time.Now().Unix())
		var traceIDBytes [16]byte
		var spanIDBytes [8]byte
		rand.Read(traceIDBytes[:])
		rand.Read(spanIDBytes[:])
		if setIds {
			span.InstrumentationLibrarySpans().At(0).Spans().At(0).SetTraceID(pdata.NewTraceID(traceIDBytes[:]))
			span.InstrumentationLibrarySpans().At(0).Spans().At(0).SetSpanID(pdata.NewSpanID(spanIDBytes[:]))
		}
	}
	return trace
}

func TestSAPMClientTokenUsageAndErrorMarshalling(t *testing.T) {
	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		translateError         bool
		sendError              bool
	}{
		{
			name:                   "no error without passthrough",
			accessTokenPassthrough: false,
			translateError:         false,
			sendError:              false,
		},
		{
			name:                   "no error with passthrough",
			accessTokenPassthrough: true,
			translateError:         false,
			sendError:              false,
		},
		{
			name:                   "translate error",
			accessTokenPassthrough: true,
			translateError:         true,
			sendError:              false,
		},
		{
			name:                   "sendError",
			accessTokenPassthrough: true,
			translateError:         false,
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
				if !tt.translateError {
					assert.True(t, tracesReceived, "Test server never received traces.")
				} else {
					assert.False(t, tracesReceived, "Test server received traces when none expected.")
				}
			}()
			defer server.Close()

			config := &Config{
				Endpoint:    server.URL,
				AccessToken: "ClientAccessToken",
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: tt.accessTokenPassthrough,
				},
			}
			params := component.ExporterCreateParams{Logger: zap.NewNop()}

			se, err := newSAPMExporter(config, params)
			assert.Nil(t, err)
			assert.NotNil(t, se, "failed to create trace exporter")

			trace := buildTestTrace(!tt.translateError)
			dropped, err := se.pushTraceData(context.Background(), trace)

			if tt.sendError || tt.translateError {
				assert.Equal(t, 2, dropped)
				require.Error(t, err)
			} else {
				assert.Equal(t, 0, dropped)
				require.NoError(t, err)
			}
		})
	}
}
