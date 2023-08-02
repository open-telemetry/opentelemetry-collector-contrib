// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

// Common structure for all the Tests
type metricTestCase struct {
	name               string
	inputAttributes    map[string]interface{}
	expectedAttributes map[string]interface{}
}

// runIndividualMetricTestCase is the common logic of passing metric data through a configured attributes processor.
func runIndividualMetricTestCase(t *testing.T, mt metricTestCase, mp processor.Metrics) {
	t.Run(mt.name, func(t *testing.T) {
		md := generateMetricData(mt.name, mt.inputAttributes)
		assert.NoError(t, mp.ConsumeMetrics(context.Background(), md))
		require.NoError(t, pmetrictest.CompareMetrics(generateMetricData(mt.name, mt.expectedAttributes), md))
	})
}

func generateMetricData(resourceName string, attrs map[string]interface{}) pmetric.Metrics {
	md := pmetric.NewMetrics()
	res := md.ResourceMetrics().AppendEmpty()
	res.Resource().Attributes().PutStr("name", resourceName)
	sl := res.ScopeMetrics().AppendEmpty()
	m := sl.Metrics().AppendEmpty()
	m.SetName("metric1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().FromRaw(attrs) //nolint:errcheck
	dp.SetIntValue(1)
	return md
}

// TestMetricProcessor_Values tests all possible value types.
func TestMetricProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyMetricTestCase struct {
		name   string
		input  pmetric.Metrics
		output pmetric.Metrics
	}
	// TODO: Add test for "nil" Metric/Attributes. This needs support from data slices to allow to construct that.
	metricTestCases := []nilEmptyMetricTestCase{
		{
			name:   "empty",
			input:  pmetric.NewMetrics(),
			output: pmetric.NewMetrics(),
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

	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), oCfg, consumertest.NewNop())
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
	t.Skip("Will be fixed by https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/17017")
	testCases := []metricTestCase{
		{
			name:            "apply processor",
			inputAttributes: map[string]interface{}{},
			expectedAttributes: map[string]interface{}{
				"attribute1": 123,
			},
		},
		{
			name: "apply processor with different value for exclude property",
			inputAttributes: map[string]interface{}{
				"NoModification": false,
			},
			expectedAttributes: map[string]interface{}{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect name for include property",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
		},
		{
			name: "attribute match for exclude property",
			inputAttributes: map[string]interface{}{
				"NoModification": true,
			},
			expectedAttributes: map[string]interface{}{
				"NoModification": true,
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
	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}

func TestAttributes_FilterMetricsByNameStrict(t *testing.T) {
	t.Skip("Will be fixed by https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/17017")
	testCases := []metricTestCase{
		{
			name:            "apply",
			inputAttributes: map[string]interface{}{},
			expectedAttributes: map[string]interface{}{
				"attribute1": 123,
			},
		},
		{
			name: "apply",
			inputAttributes: map[string]interface{}{
				"NoModification": false,
			},
			expectedAttributes: map[string]interface{}{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_metric_name",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
		},
		{
			name: "incorrect_metric_name_with_attr",
			inputAttributes: map[string]interface{}{
				"NoModification": true,
			},
			expectedAttributes: map[string]interface{}{
				"NoModification": true,
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
	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, err)
	require.NotNil(t, mp)

	for _, tc := range testCases {
		runIndividualMetricTestCase(t, tc, mp)
	}
}

func TestAttributes_FilterMetricsByNameRegexp(t *testing.T) {
	t.Skip("Will be fixed by https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/17017")
	testCases := []metricTestCase{
		{
			name:            "apply_to_metric_with_no_attrs",
			inputAttributes: map[string]interface{}{},
			expectedAttributes: map[string]interface{}{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_metric_with_attr",
			inputAttributes: map[string]interface{}{
				"NoModification": false,
			},
			expectedAttributes: map[string]interface{}{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_metric_name",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
		},
		{
			name:               "apply_dont_apply",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
		},
		{
			name: "incorrect_metric_name_with_attr",
			inputAttributes: map[string]interface{}{
				"NoModification": true,
			},
			expectedAttributes: map[string]interface{}{
				"NoModification": true,
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
	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
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
			inputAttributes: map[string]interface{}{
				"user.email": "john.doe@example.com",
			},
			expectedAttributes: map[string]interface{}{
				"user.email": "836f82db99121b3481011f16b49dfa5fbc714a0d1b1b9f784a1ebbbf5b39577f",
			},
		},
		{
			name: "Int",
			inputAttributes: map[string]interface{}{
				"user.id": 10,
			},
			expectedAttributes: map[string]interface{}{
				"user.id": "a111f275cc2e7588000001d300a31e76336d15b9d314cd1a1d8f3d3556975eed",
			},
		},
		{
			name: "Double",
			inputAttributes: map[string]interface{}{
				"user.balance": 99.1,
			},
			expectedAttributes: map[string]interface{}{
				"user.balance": "05fabd78b01be9692863cb0985f600c99da82979af18db5c55173c2a30adb924",
			},
		},
		{
			name: "Bool",
			inputAttributes: map[string]interface{}{
				"user.authenticated": true,
			},
			expectedAttributes: map[string]interface{}{
				"user.authenticated": "4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a",
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

	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
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
			inputAttributes: map[string]interface{}{
				"to.int": "123",
			},
			expectedAttributes: map[string]interface{}{
				"to.int": 123,
			},
		},
		{
			name: "String to int (bad)",
			inputAttributes: map[string]interface{}{
				"to.int": "int-10",
			},
			expectedAttributes: map[string]interface{}{
				"to.int": "int-10",
			},
		},
		{
			name: "String to double",
			inputAttributes: map[string]interface{}{
				"to.double": "3.141e2",
			},
			expectedAttributes: map[string]interface{}{
				"to.double": 314.1,
			},
		},
		{
			name: "Double to string",
			inputAttributes: map[string]interface{}{
				"to.string": 99.1,
			},
			expectedAttributes: map[string]interface{}{
				"to.string": "99.1",
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

	tp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
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
			inputAttributes: map[string]interface{}{},
			expectedAttributes: map[string]interface{}{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_metric_with_attr",
			inputAttributes: map[string]interface{}{
				"NoModification": false,
			},
			expectedAttributes: map[string]interface{}{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]interface{}{},
			expectedAttributes: map[string]interface{}{},
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
	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NoError(b, err)
	require.NotNil(b, mp)

	for _, tc := range testCases {
		md := generateMetricData(tc.name, tc.inputAttributes)

		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, mp.ConsumeMetrics(context.Background(), md))
			}
		})

		require.NoError(b, pmetrictest.CompareMetrics(generateMetricData(tc.name, tc.expectedAttributes), md))
	}
}
