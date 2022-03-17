// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package attributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Common structure for all the Tests
type metricTestCase struct {
	name               string
	inputAttributes    map[string]pdata.AttributeValue
	expectedAttributes map[string]pdata.AttributeValue
}

// runIndividualMetricTestCase is the common logic of passing metric data through a configured attributes processor.
func runIndividualMetricTestCase(t *testing.T, mt metricTestCase, mp component.MetricsProcessor) {
	t.Run(mt.name, func(t *testing.T) {
		md := generateMetricData(mt.name, mt.inputAttributes)
		assert.NoError(t, mp.ConsumeMetrics(context.Background(), md))
		// Ensure that the modified `md` has the attributes sorted:
		sortMetricAttributes(md)
		require.Equal(t, generateMetricData(mt.name, mt.expectedAttributes), md)
	})
}

func generateMetricData(resourceName string, attrs map[string]pdata.AttributeValue) pdata.Metrics {
	md := pdata.NewMetrics()
	res := md.ResourceMetrics().AppendEmpty()
	res.Resource().Attributes().InsertString("name", resourceName)
	ill := res.InstrumentationLibraryMetrics().AppendEmpty()
	m := ill.Metrics().AppendEmpty()

	switch m.DataType() {
	case pdata.MetricDataTypeGauge:
		dps := m.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			pdata.NewAttributeMapFromMap(attrs).CopyTo(dps.At(i).Attributes())
			dps.At(i).Attributes().Sort()
		}
	case pdata.MetricDataTypeSum:
		dps := m.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			pdata.NewAttributeMapFromMap(attrs).CopyTo(dps.At(i).Attributes())
			dps.At(i).Attributes().Sort()
		}
	case pdata.MetricDataTypeHistogram:
		dps := m.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			pdata.NewAttributeMapFromMap(attrs).CopyTo(dps.At(i).Attributes())
			dps.At(i).Attributes().Sort()
		}
	case pdata.MetricDataTypeExponentialHistogram:
		dps := m.ExponentialHistogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			pdata.NewAttributeMapFromMap(attrs).CopyTo(dps.At(i).Attributes())
			dps.At(i).Attributes().Sort()
		}
	case pdata.MetricDataTypeSummary:
		dps := m.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			pdata.NewAttributeMapFromMap(attrs).CopyTo(dps.At(i).Attributes())
			dps.At(i).Attributes().Sort()
		}
	}

	return md
}

func sortMetricAttributes(md pdata.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		rs.Resource().Attributes().Sort()
		ilms := rs.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			metrics := ilms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)

				switch m.DataType() {
				case pdata.MetricDataTypeGauge:
					dps := m.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).Attributes().Sort()
					}
				case pdata.MetricDataTypeSum:
					dps := m.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).Attributes().Sort()
					}
				case pdata.MetricDataTypeHistogram:
					dps := m.Histogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).Attributes().Sort()
					}
				case pdata.MetricDataTypeExponentialHistogram:
					dps := m.ExponentialHistogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).Attributes().Sort()
					}
				case pdata.MetricDataTypeSummary:
					dps := m.Summary().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).Attributes().Sort()
					}
				}
			}
		}
	}
}

// TestMetricProcessor_Values tests all possible value types.
func TestMetricProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyMetricTestCase struct {
		name   string
		input  pdata.Metrics
		output pdata.Metrics
	}
	// TODO: Add test for "nil" Metric/Attributes. This needs support from data slices to allow to construct that.
	metricTestCases := []nilEmptyMetricTestCase{
		{
			name:   "empty",
			input:  pdata.NewMetrics(),
			output: pdata.NewMetrics(),
		},
		{
			name:   "one-empty-resource-metrics",
			input:  testdata.GenerateMetricsOneEmptyResourceMetrics(),
			output: testdata.GenerateMetricsOneEmptyResourceMetrics(),
		},
		{
			name:   "no-libraries",
			input:  testdata.GenerateMetricsNoLibraries(),
			output: testdata.GenerateMetricsNoLibraries(),
		},
		{
			name:   "one-empty-instrumentation-library",
			input:  testdata.GenerateMetricsOneEmptyInstrumentationLibrary(),
			output: testdata.GenerateMetricsOneEmptyInstrumentationLibrary(),
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Settings.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
		{Key: "attribute1", Action: attraction.DELETE},
	}

	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), oCfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, mp)
	for i := range metricTestCases {
		tc := metricTestCases[i]
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, mp.ConsumeMetrics(context.Background(), tc.input))
			assert.EqualValues(t, tc.output, tc.input)
		})
	}
}

func TestAttributes_FilterMetrics(t *testing.T) {
	testCases := []metricTestCase{
		{
			name:            "apply processor",
			inputAttributes: map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1": pdata.NewAttributeValueInt(123),
			},
		},
		{
			name: "apply processor with different value for exclude property",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(false),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1":     pdata.NewAttributeValueInt(123),
				"NoModification": pdata.NewAttributeValueBool(false),
			},
		},
		{
			name:               "incorrect name for include property",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
		{
			name: "attribute match for exclude property",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^[^i].*"}},
		Config:    *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Attributes: []filterconfig.Attribute{
			{Key: "NoModification", Value: true},
		},
		Config: *createConfig(filterset.Strict),
	}
	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}

func TestAttributes_FilterMetricsByNameStrict(t *testing.T) {
	testCases := []metricTestCase{
		{
			name:            "apply",
			inputAttributes: map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1": pdata.NewAttributeValueInt(123),
			},
		},
		{
			name: "apply",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(false),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1":     pdata.NewAttributeValueInt(123),
				"NoModification": pdata.NewAttributeValueBool(false),
			},
		},
		{
			name:               "incorrect_metric_name",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
		{
			name: "incorrect_metric_name_with_attr",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "apply"}},
		Config:    *createConfig(filterset.Strict),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "dont_apply"}},
		Config:    *createConfig(filterset.Strict),
	}
	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}

func TestAttributes_FilterMetricsByNameRegexp(t *testing.T) {
	testCases := []metricTestCase{
		{
			name:            "apply_to_metric_with_no_attrs",
			inputAttributes: map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1": pdata.NewAttributeValueInt(123),
			},
		},
		{
			name: "apply_to_metric_with_attr",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(false),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1":     pdata.NewAttributeValueInt(123),
				"NoModification": pdata.NewAttributeValueBool(false),
			},
		},
		{
			name:               "incorrect_metric_name",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
		{
			name:               "apply_dont_apply",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
		{
			name: "incorrect_metric_name_with_attr",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(true),
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^apply.*"}},
		Config:    *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: ".*dont_apply$"}},
		Config:    *createConfig(filterset.Regexp),
	}
	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}

func TestMetricAttributes_Hash(t *testing.T) {
	testCases := []metricTestCase{
		{
			name: "String",
			inputAttributes: map[string]pdata.AttributeValue{
				"user.email": pdata.NewAttributeValueString("john.doe@example.com"),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"user.email": pdata.NewAttributeValueString("73ec53c4ba1747d485ae2a0d7bfafa6cda80a5a9"),
			},
		},
		{
			name: "Int",
			inputAttributes: map[string]pdata.AttributeValue{
				"user.id": pdata.NewAttributeValueInt(10),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"user.id": pdata.NewAttributeValueString("71aa908aff1548c8c6cdecf63545261584738a25"),
			},
		},
		{
			name: "Double",
			inputAttributes: map[string]pdata.AttributeValue{
				"user.balance": pdata.NewAttributeValueDouble(99.1),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"user.balance": pdata.NewAttributeValueString("76429edab4855b03073f9429fd5d10313c28655e"),
			},
		},
		{
			name: "Bool",
			inputAttributes: map[string]pdata.AttributeValue{
				"user.authenticated": pdata.NewAttributeValueBool(true),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"user.authenticated": pdata.NewAttributeValueString("bf8b4530d8d246dd74ac53a13471bba17941dff7"),
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "user.email", Action: attraction.HASH},
		{Key: "user.id", Action: attraction.HASH},
		{Key: "user.balance", Action: attraction.HASH},
		{Key: "user.authenticated", Action: attraction.HASH},
	}

	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}
func TestMetricAttributes_Convert(t *testing.T) {
	testCases := []metricTestCase{
		{
			name: "String to int (good)",
			inputAttributes: map[string]pdata.AttributeValue{
				"to.int": pdata.NewAttributeValueString("123"),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"to.int": pdata.NewAttributeValueInt(123),
			},
		},
		{
			name: "String to int (bad)",
			inputAttributes: map[string]pdata.AttributeValue{
				"to.int": pdata.NewAttributeValueString("int-10"),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"to.int": pdata.NewAttributeValueString("int-10"),
			},
		},
		{
			name: "String to double",
			inputAttributes: map[string]pdata.AttributeValue{
				"to.double": pdata.NewAttributeValueString("3.141e2"),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"to.double": pdata.NewAttributeValueDouble(314.1),
			},
		},
		{
			name: "Double to string",
			inputAttributes: map[string]pdata.AttributeValue{
				"to.string": pdata.NewAttributeValueDouble(99.1),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"to.string": pdata.NewAttributeValueString("99.1"),
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "to.int", Action: attraction.CONVERT, ConvertedType: "int"},
		{Key: "to.double", Action: attraction.CONVERT, ConvertedType: "double"},
		{Key: "to.string", Action: attraction.CONVERT, ConvertedType: "string"},
	}

	tp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualMetricTestCase(t, tt, tp)
	}
}

func BenchmarkAttributes_FilterMetricsByName(b *testing.B) {
	testCases := []metricTestCase{
		{
			name:            "apply_to_metric_with_no_attrs",
			inputAttributes: map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1": pdata.NewAttributeValueInt(123),
			},
		},
		{
			name: "apply_to_metric_with_attr",
			inputAttributes: map[string]pdata.AttributeValue{
				"NoModification": pdata.NewAttributeValueBool(false),
			},
			expectedAttributes: map[string]pdata.AttributeValue{
				"attribute1":     pdata.NewAttributeValueInt(123),
				"NoModification": pdata.NewAttributeValueBool(false),
			},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]pdata.AttributeValue{},
			expectedAttributes: map[string]pdata.AttributeValue{},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Config:    *createConfig(filterset.Regexp),
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^apply.*"}},
	}
	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(b, err)
	require.NotNil(b, mp)

	for _, tc := range testCases {
		md := generateMetricData(tc.name, tc.inputAttributes)

		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, mp.ConsumeMetrics(context.Background(), md))
			}
		})

		// Ensure that the modified `md` has the attributes sorted:
		sortMetricAttributes(md)
		require.Equal(b, generateMetricData(tc.name, tc.expectedAttributes), md)
	}
}
