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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
)

type spanEventNameTest struct {
	name     string
	inc      *SpanEventMatchProperties
	exc      *SpanEventMatchProperties
	inTraces pdata.Traces
	outSES   []string // output span events
}

type spanEventWithAttributes struct {
	spanEventName   string
	eventAttributes map[string]pdata.AttributeValue
}

var (
	inSpanEventNames = "random"

	inSpanEventForResourceTest = []spanEventWithAttributes{
		{
			spanEventName: "event",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
				"attr2": pdata.NewAttributeValueString("attr2/val2"),
				"attr3": pdata.NewAttributeValueString("attr3/val3"),
			},
		},
	}

	inSpanEventForTwoAttributes = []spanEventWithAttributes{
		{
			spanEventName: "event1",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
			},
		},
		{
			spanEventName: "event2",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val2"),
			},
		},
	}

	inSpanEventForThreeAttributes = []spanEventWithAttributes{
		{
			spanEventName: "event1",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
			},
		},
		{
			spanEventName: "event2",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val2"),
			},
		},
		{
			spanEventName: "event3",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val5"),
			},
		},
	}

	inSpanEventForFourAttributes = []spanEventWithAttributes{
		{
			spanEventName: "event1",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val1"),
			},
		},
		{
			spanEventName: "event2",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val2"),
			},
		},
		{
			spanEventName: "event3",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val3"),
			},
		},
		{
			spanEventName: "event4",
			eventAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val4"),
			},
		},
	}

	standardSpanEventTests = []spanEventNameTest{
		{
			name:     "emptyFilterInclude",
			inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: inSpanEventNames}}),
			outSES:   []string{inSpanEventNames},
		},
		{
			name:     "emptyFilterExclude",
			exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: inSpanEventNames}}),
			outSES:   []string{inSpanEventNames},
		},
		{
			name:     "emptyFilterIncludeAndExclude",
			inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: inSpanEventNames}}),
			outSES:   []string{inSpanEventNames},
		},
		{
			name:     "includeAllWithMissingAttributes",
			inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			inTraces: testSpanEvents(inSpanEventForTwoAttributes),
			outSES: []string{
				"event1",
				"event2",
			},
		},
		// {
		// 	name:     "emptyFilterExclude",
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventNames: inSpanEventNames}}),
		// 	outSES:   [][]string{inSpanEventNames},
		// },
		// {
		// 	name:     "excludeNilWithResourceAttributes",
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	inTraces: testSpanEvents(inSpanEventForResourceTest),
		// 	outSES: [][]string{
		// 		{"log1", "log2"},
		// 	},
		// },
		// {
		// 	name:     "excludeAllWithMissingResourceAttributes",
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
		// 	inTraces: testSpanEvents(inSpanEventForTwoResource),
		// 	outSES: [][]string{
		// 		{"log3", "log4"},
		// 	},
		// },
		// {
		// 	name:     "emptyFilterIncludeAndExclude",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventNames: inSpanEventNames}}),
		// 	outSES:   [][]string{inSpanEventNames},
		// },
		// {
		// 	name:     "nilWithResourceAttributesIncludeAndExclude",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
		// 	inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventNames: inSpanEventNames}}),
		// 	outSES:   [][]string{inSpanEventNames},
		// },
		// {
		// 	name:     "allWithMissingResourceAttributesIncludeAndExclude",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
		// 	inTraces: testSpanEvents(inSpanEventForTwoResource),
		// 	outSES: [][]string{
		// 		{"log3", "log4"},
		// 	},
		// },
		// {
		// 	name:     "matchAttributesWithRegexpInclude",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Regexp, EventAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val2"}}},
		// 	inTraces: testSpanEvents(inSpanEventForFourResource),
		// 	outSES: [][]string{
		// 		{"log2"},
		// 	},
		// },
		// {
		// 	name:     "matchAttributesWithRegexpInclude2",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Regexp, EventAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val(2|3)"}}},
		// 	inTraces: testSpanEvents(inSpanEventForFourResource),
		// 	outSES: [][]string{
		// 		{"log2"},
		// 		{"log3"},
		// 	},
		// },
		// {
		// 	name:     "matchAttributesWithRegexpInclude3",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Regexp, EventAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[234]"}}},
		// 	inTraces: testSpanEvents(inSpanEventForFourResource),
		// 	outSES: [][]string{
		// 		{"log2"},
		// 		{"log3"},
		// 		{"log4"},
		// 	},
		// },
		// {
		// 	name:     "matchAttributesWithRegexpInclude4",
		// 	inc:      &SpanEventMatchProperties{SpanEventMatchType: Regexp, EventAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val.*"}}},
		// 	inTraces: testSpanEvents(inSpanEventForFourResource),
		// 	outSES: [][]string{
		// 		{"log1"},
		// 		{"log2"},
		// 		{"log3"},
		// 		{"log4"},
		// 	},
		// },
		// {
		// 	name:     "matchAttributesWithRegexpExclude",
		// 	exc:      &SpanEventMatchProperties{SpanEventMatchType: Regexp, EventAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[23]"}}},
		// 	inTraces: testSpanEvents(inSpanEventForFourResource),
		// 	outSES: [][]string{
		// 		{"log1"},
		// 		{"log4"},
		// 	},
		// },
		// {
		// 	name: "matchRecordAttributeWithRegexp1",
		// 	inc: &SpanEventMatchProperties{
		// 		SpanEventMatchType: Regexp,
		// 		EventAttributes: []filterconfig.Attribute{
		// 			{
		// 				Key:   "rec",
		// 				Value: "rec/val[1]",
		// 			},
		// 		},
		// 	},
		// 	inTraces: testSpanEvents(inSpanEventForTwoResourceWithRecordAttributes),
		// 	outSES: [][]string{
		// 		{"log1", "log2"},
		// 	},
		// },
		// {
		// 	name: "matchRecordAttributeWithRegexp2",
		// 	inc: &SpanEventMatchProperties{
		// 		SpanEventMatchType: Regexp,
		// 		EventAttributes: []filterconfig.Attribute{
		// 			{
		// 				Key:   "rec",
		// 				Value: "rec/val[^2]",
		// 			},
		// 		},
		// 	},
		// 	inTraces: testSpanEvents(inSpanEventForTwoResourceWithRecordAttributes),
		// 	outSES: [][]string{
		// 		{"log1", "log2"},
		// 	},
		// },
		// {
		// 	name: "matchRecordAttributeWithRegexp2",
		// 	inc: &SpanEventMatchProperties{
		// 		SpanEventMatchType: Regexp,
		// 		EventAttributes: []filterconfig.Attribute{
		// 			{
		// 				Key:   "rec",
		// 				Value: "rec/val[1|2]",
		// 			},
		// 		},
		// 	},
		// 	inTraces: testSpanEvents(inSpanEventForTwoResourceWithRecordAttributes),
		// 	outSES: [][]string{
		// 		{"log1", "log2"},
		// 		{"log3", "log4"},
		// 	},
		// },
		// {
		// 	name: "matchRecordAttributeWithRegexp3",
		// 	inc: &SpanEventMatchProperties{
		// 		SpanEventMatchType: Regexp,
		// 		EventAttributes: []filterconfig.Attribute{
		// 			{
		// 				Key:   "rec",
		// 				Value: "rec/val[1|5]",
		// 			},
		// 		},
		// 	},
		// 	inTraces: testSpanEvents(inSpanEventForThreeResourceWithRecordAttributes),
		// 	outSES: [][]string{
		// 		{"log1", "log2"},
		// 		{"log5"},
		// 	},
		// },
	}
)

func TestFilterSpanEventProcessor(t *testing.T) {
	for _, test := range standardSpanEventTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter span event processor
			next := new(consumertest.TracesSink)
			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				SpanEvents: SpanEventFilters{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			flp, err := factory.CreateTracesProcessor(
				context.Background(),
				componenttest.NewNopProcessorCreateSettings(),
				cfg,
				next,
			)
			assert.NotNil(t, flp)
			assert.Nil(t, err)

			caps := flp.Capabilities()
			assert.True(t, caps.MutatesData)
			ctx := context.Background()
			assert.NoError(t, flp.Start(ctx, nil))

			cErr := flp.ConsumeTraces(context.Background(), test.inTraces)
			assert.Nil(t, cErr)
			got := next.AllTraces()

			require.Len(t, got, 1)
			rTraces := got[0].ResourceSpans()
			assert.Equal(t, 1, rTraces.Len())

			ilSpans := rTraces.At(0).InstrumentationLibrarySpans().At(0).Spans()
			gotEvents := ilSpans.At(0).Events()
			assert.Equal(t, len(test.outSES), gotEvents.Len())
			for i, spanEventName := range test.outSES {
				assert.Equal(t, spanEventName, gotEvents.At(i).Name())
			}
			assert.NoError(t, flp.Shutdown(ctx))
		})
	}
}

func testSpanEvents(sewas []spanEventWithAttributes) pdata.Traces {
	td := pdata.NewTraces()
	rSpans := td.ResourceSpans().AppendEmpty()
	ilSpans := rSpans.InstrumentationLibrarySpans().AppendEmpty()
	spans := ilSpans.Spans()
	span := spans.AppendEmpty()
	span.SetName("test")
	span.Events().EnsureCapacity(len(sewas))

	for _, sewa := range sewas {
		es := span.Events()
		e := es.AppendEmpty()
		e.SetName(sewa.spanEventName)
		e.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		e.Attributes().InitFromMap(sewa.eventAttributes)
	}

	return td
}

// func TestNilResourceLogs(t *testing.T) {
// 	logs := pdata.NewLogs()
// 	rls := logs.ResourceLogs()
// 	rls.AppendEmpty()
// 	requireNotPanicsLogs(t, logs)
// }

// func TestNilILL(t *testing.T) {
// 	logs := pdata.NewLogs()
// 	rls := logs.ResourceLogs()
// 	rl := rls.AppendEmpty()
// 	ills := rl.InstrumentationLibraryLogs()
// 	ills.AppendEmpty()
// 	requireNotPanicsLogs(t, logs)
// }

// func TestNilLog(t *testing.T) {
// 	logs := pdata.NewLogs()
// 	rls := logs.ResourceLogs()
// 	rl := rls.AppendEmpty()
// 	ills := rl.InstrumentationLibraryLogs()
// 	ill := ills.AppendEmpty()
// 	ls := ill.Logs()
// 	ls.AppendEmpty()
// 	requireNotPanicsLogs(t, logs)
// }

// func requireNotPanicsLogs(t *testing.T, logs pdata.Logs) {
// 	factory := NewFactory()
// 	cfg := factory.CreateDefaultConfig()
// 	pcfg := cfg.(*Config)
// 	pcfg.Logs = LogFilters{
// 		Exclude: nil,
// 	}
// 	ctx := context.Background()
// 	proc, _ := factory.CreateLogsProcessor(
// 		ctx,
// 		componenttest.NewNopProcessorCreateSettings(),
// 		cfg,
// 		consumertest.NewNop(),
// 	)
// 	require.NotPanics(t, func() {
// 		_ = proc.ConsumeLogs(ctx, logs)
// 	})
// }
