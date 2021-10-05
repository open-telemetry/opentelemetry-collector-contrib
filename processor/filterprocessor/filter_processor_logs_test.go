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
)

type logNameTest struct {
	name   string
	inc    *LogMatchProperties
	exc    *LogMatchProperties
	inLogs pdata.Logs
	outLN  [][]string // output Log names per Resource
}

type logWithResource struct {
	logNames           []string
	resourceAttributes map[string]pdata.AttributeValue
	recordAttributes   map[string]pdata.AttributeValue
}

var (
	inLogNames = []string{
		"full_name_match",
		"random",
	}

	inLogForResourceTest = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
				"attr2": pdata.NewAttributeValueString("attr2/val2"),
				"attr3": pdata.NewAttributeValueString("attr3/val3"),
			},
		},
	}

	inLogForTwoResource = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val2"),
			},
		},
	}

	inLogForTwoResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
			},
			recordAttributes: map[string]pdata.AttributeValue{
				"rec": pdata.NewAttributeValueString("rec/val1"),
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val2"),
			},
			recordAttributes: map[string]pdata.AttributeValue{
				"rec": pdata.NewAttributeValueString("rec/val2"),
			},
		},
	}
	inLogForThreeResourceWithRecordAttributes = []logWithResource{
		{
			logNames: []string{"log1", "log2"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val1"),
			},
			recordAttributes: map[string]pdata.AttributeValue{
				"rec": pdata.NewAttributeValueString("rec/val1"),
			},
		},
		{
			logNames: []string{"log3", "log4"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val2"),
			},
			recordAttributes: map[string]pdata.AttributeValue{
				"rec": pdata.NewAttributeValueString("rec/val2"),
			},
		},
		{
			logNames: []string{"log5"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr1": pdata.NewAttributeValueString("attr1/val5"),
			},
			recordAttributes: map[string]pdata.AttributeValue{
				"rec": pdata.NewAttributeValueString("rec/val5"),
			},
		},
	}

	inLogForFourResource = []logWithResource{
		{
			logNames: []string{"log1"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val1"),
			},
		},
		{
			logNames: []string{"log2"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val2"),
			},
		},
		{
			logNames: []string{"log3"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val3"),
			},
		},
		{
			logNames: []string{"log4"},
			resourceAttributes: map[string]pdata.AttributeValue{
				"attr": pdata.NewAttributeValueString("attr/val4"),
			},
		},
	}

	standardLogTests = []logNameTest{
		{
			name:   "emptyFilterInclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "includeNilWithResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "includeAllWithMissingResourceAttributes",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterExclude",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "excludeNilWithResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs(inLogForResourceTest),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name:   "excludeAllWithMissingResourceAttributes",
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "emptyFilterIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "nilWithResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{}},
			inLogs: testResourceLogs([]logWithResource{{logNames: inLogNames}}),
			outLN:  [][]string{inLogNames},
		},
		{
			name:   "allWithMissingResourceAttributesIncludeAndExclude",
			inc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val2"}}},
			exc:    &LogMatchProperties{LogMatchType: Strict, ResourceAttributes: []filterconfig.Attribute{{Key: "attr1", Value: "attr1/val1"}}},
			inLogs: testResourceLogs(inLogForTwoResource),
			outLN: [][]string{
				{"log3", "log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val2"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude2",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val(2|3)"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude3",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[234]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log2"},
				{"log3"},
				{"log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpInclude4",
			inc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val.*"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log1"},
				{"log2"},
				{"log3"},
				{"log4"},
			},
		},
		{
			name:   "matchAttributesWithRegexpExclude",
			exc:    &LogMatchProperties{LogMatchType: Regexp, ResourceAttributes: []filterconfig.Attribute{{Key: "attr", Value: "attr/val[23]"}}},
			inLogs: testResourceLogs(inLogForFourResource),
			outLN: [][]string{
				{"log1"},
				{"log4"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp1",
			inc: &LogMatchProperties{
				LogMatchType: Regexp,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp2",
			inc: &LogMatchProperties{
				LogMatchType: Regexp,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[^2]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp2",
			inc: &LogMatchProperties{
				LogMatchType: Regexp,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1|2]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForTwoResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
				{"log3", "log4"},
			},
		},
		{
			name: "matchRecordAttributeWithRegexp3",
			inc: &LogMatchProperties{
				LogMatchType: Regexp,
				RecordAttributes: []filterconfig.Attribute{
					{
						Key:   "rec",
						Value: "rec/val[1|5]",
					},
				},
			},
			inLogs: testResourceLogs(inLogForThreeResourceWithRecordAttributes),
			outLN: [][]string{
				{"log1", "log2"},
				{"log5"},
			},
		},
	}
)

func TestFilterLogProcessor(t *testing.T) {
	for _, test := range standardLogTests {
		t.Run(test.name, func(t *testing.T) {
			// next stores the results of the filter log processor
			next := new(consumertest.LogsSink)
			cfg := &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Logs: LogFilters{
					Include: test.inc,
					Exclude: test.exc,
				},
			}
			factory := NewFactory()
			flp, err := factory.CreateLogsProcessor(
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

			cErr := flp.ConsumeLogs(context.Background(), test.inLogs)
			assert.Nil(t, cErr)
			got := next.AllLogs()

			require.Len(t, got, 1)
			rLogs := got[0].ResourceLogs()
			assert.Equal(t, len(test.outLN), rLogs.Len())

			for i, wantOut := range test.outLN {
				gotLogs := rLogs.At(i).InstrumentationLibraryLogs().At(0).Logs()
				assert.Equal(t, len(wantOut), gotLogs.Len())
				for idx := range wantOut {
					assert.Equal(t, wantOut[idx], gotLogs.At(idx).Name())
				}
			}
			assert.NoError(t, flp.Shutdown(ctx))
		})
	}
}

func testResourceLogs(lwrs []logWithResource) pdata.Logs {
	ld := pdata.NewLogs()

	for i, lwr := range lwrs {
		rl := ld.ResourceLogs().AppendEmpty()

		// Add resource level attribtues
		rl.Resource().Attributes().InitFromMap(lwr.resourceAttributes)
		ls := rl.InstrumentationLibraryLogs().AppendEmpty().Logs()
		for _, name := range lwr.logNames {
			l := ls.AppendEmpty()
			l.SetName(name)

			// Add record level attribtues
			for k := 0; k < ls.Len(); k++ {
				ls.At(k).Attributes().InitFromMap(lwrs[i].recordAttributes)
			}
		}
	}
	return ld
}

func TestNilResourceLogs(t *testing.T) {
	logs := pdata.NewLogs()
	rls := logs.ResourceLogs()
	rls.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func TestNilILL(t *testing.T) {
	logs := pdata.NewLogs()
	rls := logs.ResourceLogs()
	rl := rls.AppendEmpty()
	ills := rl.InstrumentationLibraryLogs()
	ills.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func TestNilLog(t *testing.T) {
	logs := pdata.NewLogs()
	rls := logs.ResourceLogs()
	rl := rls.AppendEmpty()
	ills := rl.InstrumentationLibraryLogs()
	ill := ills.AppendEmpty()
	ls := ill.Logs()
	ls.AppendEmpty()
	requireNotPanicsLogs(t, logs)
}

func requireNotPanicsLogs(t *testing.T, logs pdata.Logs) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.Logs = LogFilters{
		Exclude: nil,
	}
	ctx := context.Background()
	proc, _ := factory.CreateLogsProcessor(
		ctx,
		componenttest.NewNopProcessorCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeLogs(ctx, logs)
	})
}
