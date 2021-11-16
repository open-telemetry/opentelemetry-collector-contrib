package filterprocessor

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"testing"
)

type testTrace struct {
	span_name           string
	library_name        string
	library_version     string
	resource_attributes map[string]pdata.AttributeValue
	tags                map[string]pdata.AttributeValue
}

type traceTest struct {
	name              string
	inc               *filterconfig.MatchProperties
	exc               *filterconfig.MatchProperties
	inTraces          pdata.Traces
	allTracesFiltered bool
	spanCount         int
}

var redisTraces = []testTrace{
	{
		span_name:       "test!",
		library_name:    "otel",
		library_version: "11",
		resource_attributes: map[string]pdata.AttributeValue{
			"service.name": pdata.NewAttributeValueString("test_service"),
		},
		tags: map[string]pdata.AttributeValue{
			"db.type": pdata.NewAttributeValueString("redis"),
		},
	},
}

var nameTraces = []testTrace{
	{
		span_name:       "test!",
		library_name:    "otel",
		library_version: "11",
		resource_attributes: map[string]pdata.AttributeValue{
			"service.name": pdata.NewAttributeValueString("keep"),
		},
		tags: map[string]pdata.AttributeValue{
			"db.type": pdata.NewAttributeValueString("redis"),
		},
	},
	{
		span_name:       "test!",
		library_name:    "otel",
		library_version: "11",
		resource_attributes: map[string]pdata.AttributeValue{
			"service.name": pdata.NewAttributeValueString("dont_keep"),
		},
		tags: map[string]pdata.AttributeValue{
			"db.type": pdata.NewAttributeValueString("redis"),
		},
	},
	{
		span_name:       "test!",
		library_name:    "otel",
		library_version: "11",
		resource_attributes: map[string]pdata.AttributeValue{
			"service.name": pdata.NewAttributeValueString("keep"),
		},
		tags: map[string]pdata.AttributeValue{
			"db.type": pdata.NewAttributeValueString("redis"),
		},
	},
}
var serviceNameMatchProperties = &filterconfig.MatchProperties{Config: filterset.Config{MatchType: filterset.Strict}, Services: []string{"keep"}}
var redisMatchProperties = &filterconfig.MatchProperties{Attributes: []filterconfig.Attribute{{Key: "db.type", Value: "redis"}}}
var standardTraceTests = []traceTest{
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

func TestFilterTraceProcessor(t *testing.T) {
	for _, test := range standardTraceTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter metric processor
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
func generateTraces(tests []testTrace) pdata.Traces {
	md := pdata.NewTraces()

	for _, test := range tests {
		rs := md.ResourceSpans().AppendEmpty()
		ils := rs.InstrumentationLibrarySpans().AppendEmpty()
		ils.InstrumentationLibrary().SetName(test.library_name)
		ils.InstrumentationLibrary().SetVersion(test.library_version)
		rs.Resource().Attributes().InitFromMap(test.resource_attributes)
		span := ils.Spans().AppendEmpty()
		span.Attributes().InitFromMap(test.tags)
		span.SetName(test.span_name)
	}
	return md
}
