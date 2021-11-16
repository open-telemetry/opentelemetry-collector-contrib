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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
)

type testTrace struct {
	spanName           string
	libraryName        string
	libraryVersion     string
	resourceAttributes map[string]pdata.AttributeValue
	tags               map[string]pdata.AttributeValue
}

type traceTest struct {
	name              string
	inc               *filterconfig.MatchProperties
	exc               *filterconfig.MatchProperties
	inTraces          pdata.Traces
	allTracesFiltered bool
	spanCount         int
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
			tags: map[string]pdata.AttributeValue{
				"db.type": pdata.NewAttributeValueString("redis"),
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("dont_keep"),
			},
			tags: map[string]pdata.AttributeValue{
				"db.type": pdata.NewAttributeValueString("redis"),
			},
		},
		{
			spanName:       "test!",
			libraryName:    "otel",
			libraryVersion: "11",
			resourceAttributes: map[string]pdata.AttributeValue{
				"service.name": pdata.NewAttributeValueString("keep"),
			},
			tags: map[string]pdata.AttributeValue{
				"db.type": pdata.NewAttributeValueString("redis"),
			},
		},
	}
	serviceNameMatchProperties = &filterconfig.MatchProperties{Config: filterset.Config{MatchType: filterset.Strict}, Services: []string{"keep"}}
	redisMatchProperties       = &filterconfig.MatchProperties{Attributes: []filterconfig.Attribute{{Key: "db.type", Value: "redis"}}}
	standardTraceTests         = []traceTest{
		{
			name:              "filterRedis",
			exc:               redisMatchProperties,
			inTraces:          generateTraces(redisTraces),
			allTracesFiltered: true,
		},
		{
			name:      "keepRedis",
			inc:       redisMatchProperties,
			inTraces:  generateTraces(redisTraces),
			spanCount: 1,
		},
		{
			name:      "keepServiceName",
			inc:       serviceNameMatchProperties,
			inTraces:  generateTraces(nameTraces),
			spanCount: 2,
		},
	}
)

func TestFilterTraceProcessor(t *testing.T) {
	for _, test := range standardTraceTests {
		t.Run(test.name, func(t *testing.T) {
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
				context.Background(),
				componenttest.NewNopProcessorCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, fmp)
			assert.Nil(t, err)

			caps := fmp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			assert.NoError(t, fmp.Start(ctx, nil))

			cErr := fmp.ConsumeTraces(context.Background(), test.inTraces)
			assert.Nil(t, cErr)
			got := next.AllTraces()

			if test.allTracesFiltered {
				require.Equal(t, 0, len(got))
			} else {
				require.Equal(t, test.spanCount, got[0].SpanCount())
			}
			assert.NoError(t, fmp.Shutdown(ctx))
		})
	}
}
func generateTraces(traces []testTrace) pdata.Traces {
	md := pdata.NewTraces()

	for _, test := range traces {
		rs := md.ResourceSpans().AppendEmpty()
		pdata.NewAttributeMapFromMap(test.resourceAttributes).CopyTo(rs.Resource().Attributes())
		ils := rs.InstrumentationLibrarySpans().AppendEmpty()
		ils.InstrumentationLibrary().SetName(test.libraryName)
		ils.InstrumentationLibrary().SetVersion(test.libraryVersion)
		span := ils.Spans().AppendEmpty()
		pdata.NewAttributeMapFromMap(test.tags).CopyTo(span.Attributes())
		span.SetName(test.spanName)
	}
	return md
}
