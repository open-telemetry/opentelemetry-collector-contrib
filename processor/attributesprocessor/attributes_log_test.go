// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor/internal/metadata"
)

// Common structure for all the Tests
type logTestCase struct {
	name               string
	inputAttributes    map[string]any
	expectedAttributes map[string]any
}

// runIndividualLogTestCase is the common logic of passing trace data through a configured attributes processor.
func runIndividualLogTestCase(t *testing.T, tt logTestCase, tp processor.Logs) {
	t.Run(tt.name, func(t *testing.T) {
		ld := generateLogData(tt.name, tt.inputAttributes)
		assert.NoError(t, tp.ConsumeLogs(context.Background(), ld))
		assert.NoError(t, plogtest.CompareLogs(generateLogData(tt.name, tt.expectedAttributes), ld))
	})
}

func generateLogData(resourceName string, attrs map[string]any) plog.Logs {
	td := plog.NewLogs()
	res := td.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("name", resourceName)
	sl := res.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	//nolint:errcheck
	lr.Attributes().FromRaw(attrs)
	return td
}

// TestLogProcessor_Values tests all possible value types.
func TestLogProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyTestCase struct {
		name   string
		input  plog.Logs
		output plog.Logs
	}
	testCases := []nilEmptyTestCase{
		{
			name:   "empty",
			input:  plog.NewLogs(),
			output: plog.NewLogs(),
		},
		{
			name:   "one-empty-resource-logs",
			input:  testdata.GenerateLogsOneEmptyResourceLogs(),
			output: testdata.GenerateLogsOneEmptyResourceLogs(),
		},
		{
			name:   "no-libraries",
			input:  testdata.GenerateLogsOneEmptyResourceLogs(),
			output: testdata.GenerateLogsOneEmptyResourceLogs(),
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Settings.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
		{Key: "attribute1", Action: attraction.DELETE},
	}

	tp, err := factory.CreateLogs(
		context.Background(), processortest.NewNopSettings(metadata.Type), oCfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tp.ConsumeLogs(context.Background(), tt.input))
			assert.Equal(t, tt.output, tt.input)
		})
	}
}

func TestAttributes_FilterLogs(t *testing.T) {
	testCases := []logTestCase{
		{
			name:            "apply processor",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply processor with different value for exclude property",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect name for include property",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "attribute match for exclude property",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
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
		// Libraries: []filterconfig.InstrumentationLibrary{{Name: "^[^i].*"}},
		Config: *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Attributes: []filterconfig.Attribute{
			{Key: "NoModification", Value: true},
		},
		Config: *createConfig(filterset.Strict),
	}
	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterLogsByNameStrict(t *testing.T) {
	testCases := []logTestCase{
		{
			name:            "apply",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_log_name",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "incorrect_log_name_with_attr",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
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
	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterLogsByNameRegexp(t *testing.T) {
	testCases := []logTestCase{
		{
			name:            "apply_to_log_with_no_attrs",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_log_with_attr",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_log_name",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "apply_dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "incorrect_log_name_with_attr",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
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
	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}

func TestLogAttributes_Hash(t *testing.T) {
	testCases := []logTestCase{
		{
			name: "String",
			inputAttributes: map[string]any{
				"user.email": "john.doe@example.com",
			},
			expectedAttributes: map[string]any{
				"user.email": "836f82db99121b3481011f16b49dfa5fbc714a0d1b1b9f784a1ebbbf5b39577f",
			},
		},
		{
			name: "Int",
			inputAttributes: map[string]any{
				"user.id": 10,
			},
			expectedAttributes: map[string]any{
				"user.id": "a111f275cc2e7588000001d300a31e76336d15b9d314cd1a1d8f3d3556975eed",
			},
		},
		{
			name: "Double",
			inputAttributes: map[string]any{
				"user.balance": 99.1,
			},
			expectedAttributes: map[string]any{
				"user.balance": "05fabd78b01be9692863cb0985f600c99da82979af18db5c55173c2a30adb924",
			},
		},
		{
			name: "Bool",
			inputAttributes: map[string]any{
				"user.authenticated": true,
			},
			expectedAttributes: map[string]any{
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

	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}

func TestLogAttributes_Convert(t *testing.T) {
	testCases := []logTestCase{
		{
			name: "int to int",
			inputAttributes: map[string]any{
				"to.int": 1,
			},
			expectedAttributes: map[string]any{
				"to.int": 1,
			},
		},
		{
			name: "false to int",
			inputAttributes: map[string]any{
				"to.int": false,
			},
			expectedAttributes: map[string]any{
				"to.int": 0,
			},
		},
		{
			name: "String to int (good)",
			inputAttributes: map[string]any{
				"to.int": "123",
			},
			expectedAttributes: map[string]any{
				"to.int": 123,
			},
		},
		{
			name: "String to int (bad)",
			inputAttributes: map[string]any{
				"to.int": "int-10",
			},
			expectedAttributes: map[string]any{
				"to.int": "int-10",
			},
		},
		{
			name: "String to double",
			inputAttributes: map[string]any{
				"to.double": "123.6",
			},
			expectedAttributes: map[string]any{
				"to.double": 123.6,
			},
		},
		{
			name: "Double to string",
			inputAttributes: map[string]any{
				"to.string": 99.1,
			},
			expectedAttributes: map[string]any{
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

	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualLogTestCase(t, tt, tp)
	}
}

func BenchmarkAttributes_FilterLogsByName(b *testing.B) {
	testCases := []logTestCase{
		{
			name:            "apply_to_log_with_no_attrs",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_log_with_attr",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
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
	tp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(b, err)
	require.NotNil(b, tp)

	for _, tt := range testCases {
		td := generateLogData(tt.name, tt.inputAttributes)

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, tp.ConsumeLogs(context.Background(), td))
			}
		})

		require.NoError(b, plogtest.CompareLogs(generateLogData(tt.name, tt.expectedAttributes), td))
	}
}
