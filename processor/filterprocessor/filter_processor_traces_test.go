// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

// All the data we need to test the Span filter
type testTrace struct {
	spanName           string
	libraryName        string
	libraryVersion     string
	resourceAttributes map[string]pdata.AttributeValue
	tags               map[string]pdata.AttributeValue
}

// All the data we need to define a test
type traceTest struct {
	name              string
	inc               *filterconfig.MatchProperties
	exc               *filterconfig.MatchProperties
	inTraces          pdata.Traces
	allTracesFiltered bool
	spanCountExpected int // The number of spans that should be left after all filtering
}

var (
	redisTraces = []testTrace{
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("test_service"),
			},
			tags: map[string]pdata.AttributeValue{
				"db.type": pdata.NewAttributeValueString("redis"),
			},
		},
	}

	nameTraces = []testTrace{
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("keep"),
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("dont_keep"),
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("keep"),
			},
		},
	}

	serviceNameMatchProperties = &filterconfig.MatchProperties{
		Config:   filterset.Config{MatchType: filterset.Strict},
		Services: []string{"keep"},
	}

	redisMatchProperties = &filterconfig.MatchProperties{
		Attributes: []filterconfig.Attribute{
			{Key: "db.type", Value: "redis"},
		},
	}

	standardTraceTests = []traceTest{
		{
			name:              "filterRedis",
			exc:               redisMatchProperties,
			inTraces:          generateTraces(redisTraces),
			allTracesFiltered: true,
		},
		{
			name:              "keepRedis",
			inc:               redisMatchProperties,
			inTraces:          generateTraces(redisTraces),
			spanCountExpected: 1,
		},
		{
			name:              "keepServiceName",
			inc:               serviceNameMatchProperties,
			inTraces:          generateTraces(nameTraces),
			spanCountExpected: 2,
		},
	}
)

func TestFilterTraceProcessor(t *testing.T) {
	for _, test := range standardTraceTests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			next := new(consumertest.TracesSink)
			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Spans: SpanFilters{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			fmp, err := factory.CreateTracesProcessor(
				ctx,
				componenttest.NewNopProcessorCreateSettings(),
				cfg,
				next,
			)
			require.NotNil(t, fmp)
			require.Nil(t, err)

			caps := fmp.Capabilities()
			require.True(t, caps.MutatesData)

			require.NoError(t, fmp.Start(ctx, nil))

			cErr := fmp.ConsumeTraces(ctx, test.inTraces)
			require.Nil(t, cErr)
			got := next.AllTraces()

			// If all traces got filtered you shouldn't even have ResourceSpans
			if test.allTracesFiltered {
				require.Equal(t, 0, len(got))
			} else {
				require.Equal(t, test.spanCountExpected, got[0].SpanCount())
			}
			require.NoError(t, fmp.Shutdown(ctx))
		})
	}
}
func generateTraces(traces []testTrace) pdata.Traces {
	td := pdata.NewTraces()

	for _, trace := range traces {
		rs := td.ResourceSpans().AppendEmpty()
		pdata.NewAttributeMapFromMap(trace.resourceAttributes).CopyTo(rs.Resource().Attributes())
		ils := rs.InstrumentationLibrarySpans().AppendEmpty()
		ils.InstrumentationLibrary().SetName(trace.libraryName)
		ils.InstrumentationLibrary().SetVersion(trace.libraryVersion)
		span := ils.Spans().AppendEmpty()
		pdata.NewAttributeMapFromMap(trace.tags).CopyTo(span.Attributes())
		span.SetName(trace.spanName)
	}
	return td
}
