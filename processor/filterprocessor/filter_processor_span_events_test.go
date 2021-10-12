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
	standardSpanEventTests = []spanEventNameTest{
		{
			name:     "emptyIncludeFilterShouldProcessAll",
			inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: "random"}}),
			outSES:   []string{"random"},
		},
		{
			name:     "emptyExcludeFilterShouldProcessAll",
			exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: "random"}}),
			outSES:   []string{"random"},
		},
		{
			name:     "emptyIncludeAndExcludeFilterShouldIncludeAll",
			inc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			exc:      &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{}},
			inTraces: testSpanEvents([]spanEventWithAttributes{{spanEventName: "random"}}),
			outSES:   []string{"random"},
		},
		{
			name: "shouldOnlyIncludeEventsWithMatchingAttributes",
			inc:  &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inTraces: testSpanEvents([]spanEventWithAttributes{
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
			}),
			outSES: []string{
				"event1",
			},
		},
		{
			name: "shouldOnlyExcludeEventsWithMatchingAttributes",
			exc:  &SpanEventMatchProperties{SpanEventMatchType: Strict, EventAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inTraces: testSpanEvents([]spanEventWithAttributes{
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
			}),
			outSES: []string{
				"event2",
			},
		},
		{
			name: "shouldMatchIncludeAttributesFirstAndThenExclude",
			inc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventAttributes: []filterconfig.Attribute{
					{Key: "attr1", Value: "val1"},
					{Key: "attr2", Value: "val2"},
					{Key: "attr3", Value: "val3"},
				},
			},
			exc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventAttributes: []filterconfig.Attribute{
					{Key: "attr4", Value: "val4"},
				},
			},
			inTraces: testSpanEvents([]spanEventWithAttributes{
				{
					spanEventName: "event1",
					eventAttributes: map[string]pdata.AttributeValue{
						"attr1": pdata.NewAttributeValueString("val1"),
						"attr2": pdata.NewAttributeValueString("val2"),
						"attr3": pdata.NewAttributeValueString("val3"),
						"attr4": pdata.NewAttributeValueString("val4"),
					},
				},
				{
					spanEventName: "event2",
					eventAttributes: map[string]pdata.AttributeValue{
						"attr1": pdata.NewAttributeValueString("val1"),
						"attr2": pdata.NewAttributeValueString("val2"),
						"attr3": pdata.NewAttributeValueString("val3"),
					},
				},
			}),
			outSES: []string{
				"event2",
			},
		},
		{
			name: "shouldIncludeMatchedNamesIncludeFilter",
			inc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventNames:         []string{"event1", "event2"},
			},
			inTraces: testSpanEvents([]spanEventWithAttributes{
				{
					spanEventName: "event1",
				},
				{
					spanEventName: "event2",
				},
			}),
			outSES: []string{
				"event1",
				"event2",
			},
		},
		{
			name: "shouldExcludeMatchedNamesExcludeFilter",
			exc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventNames:         []string{"event1"},
			},
			inTraces: testSpanEvents([]spanEventWithAttributes{
				{
					spanEventName: "event1",
				},
				{
					spanEventName: "event2",
				},
			}),
			outSES: []string{
				"event2",
			},
		},
		{
			name: "shouldMatchIncludedNamesFirstAndThenExcludedNames",
			inc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventNames:         []string{"event1", "event2", "event3"},
			},
			exc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventNames:         []string{"event2"},
			},
			inTraces: testSpanEvents([]spanEventWithAttributes{
				{
					spanEventName: "event1",
				},
				{
					spanEventName: "event2",
				},
				{
					spanEventName: "event3",
				},
				{
					spanEventName: "event4",
				},
			}),
			outSES: []string{
				"event1",
				"event3",
			},
		},
		{
			name: "shouldMatchIncludedNamesFirstAndThenExcludedNamesRexex",
			inc: &SpanEventMatchProperties{
				SpanEventMatchType: Regexp,
				EventNames:         []string{"event(1|2|3)"},
			},
			exc: &SpanEventMatchProperties{
				SpanEventMatchType: Strict,
				EventNames:         []string{"event2"},
			},
			inTraces: testSpanEvents([]spanEventWithAttributes{
				{
					spanEventName: "event1",
				},
				{
					spanEventName: "event2",
				},
				{
					spanEventName: "event3",
				},
				{
					spanEventName: "event4",
				},
			}),
			outSES: []string{
				"event1",
				"event3",
			},
		},
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

func TestNilResourceTraces(t *testing.T) {
	traces := pdata.NewTraces()
	rss := traces.ResourceSpans()
	rss.AppendEmpty()
	requireNotPanicsTraces(t, traces)
}

func TestNilILS(t *testing.T) {
	traces := pdata.NewTraces()
	rss := traces.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.InstrumentationLibrarySpans()
	ils.AppendEmpty()
	requireNotPanicsTraces(t, traces)
}

func TestNilTraces(t *testing.T) {
	traces := pdata.NewTraces()
	rs := traces.ResourceSpans()
	r := rs.AppendEmpty()
	ils := r.InstrumentationLibrarySpans()
	is := ils.AppendEmpty()
	ss := is.Spans()
	ss.AppendEmpty()
	requireNotPanicsTraces(t, traces)
}

func requireNotPanicsTraces(t *testing.T, traces pdata.Traces) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.SpanEvents = SpanEventFilters{
		Exclude: nil,
	}
	ctx := context.Background()
	proc, _ := factory.CreateTracesProcessor(
		ctx,
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeTraces(ctx, traces)
	})
}
