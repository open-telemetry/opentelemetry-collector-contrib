// Copyright 2020, OpenTelemetry Authors
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func TestCreateTraceExporter(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "sapm/customname"},
		Endpoint:         "test-endpoint",
		AccessToken:      "abcd1234",
		NumWorkers:       3,
		MaxConnections:   45,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newSAPMTraceExporter(config, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
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
			fmt.Printf("tt.useToken: %v", tt.useToken)
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
