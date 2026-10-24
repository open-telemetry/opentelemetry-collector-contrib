// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "tail_sampling_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t,
		&Config{
			DecisionWait:            10 * time.Second,
			NumTraces:               100,
			NumShards:               1,
			ExpectedNewTracesPerSec: 10,
			SamplingStrategy:        samplingStrategyTraceComplete,
			DecisionCache:           DecisionCacheConfig{SampledCacheSize: 1_000, NonSampledCacheSize: 10_000},
			PolicyCfgs: []PolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-policy-1",
						Type: AlwaysSample,
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-policy-2",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 5000},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:                "test-policy-3",
						Type:                NumericAttribute,
						NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:             "test-policy-4",
						Type:             Probabilistic,
						ProbabilisticCfg: ProbabilisticCfg{HashSalt: "custom-salt", SamplingPercentage: 0.1},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:          "test-policy-5",
						Type:          StatusCode,
						StatusCodeCfg: StatusCodeCfg{StatusCodes: []string{"ERROR", "UNSET"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:               "test-policy-6",
						Type:               StringAttribute,
						StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:            "test-policy-7",
						Type:            RateLimiting,
						RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 35, BurstCapacity: 70},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:             "test-policy-8",
						Type:             BytesLimiting,
						BytesLimitingCfg: BytesLimitingCfg{BytesPerSecond: 1024000, BurstCapacity: 2048000},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:         "test-policy-9",
						Type:         SpanCount,
						SpanCountCfg: SpanCountCfg{MinSpans: 2},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:          "test-policy-10",
						Type:          TraceState,
						TraceStateCfg: TraceStateCfg{Key: "key3", Values: []string{"value1", "value2"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:                "test-policy-11",
						Type:                BooleanAttribute,
						BooleanAttributeCfg: BooleanAttributeCfg{Key: "key4", Value: true},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-policy-12",
						Type: OTTLCondition,
						OTTLConditionCfg: OTTLConditionCfg{
							ErrorMode:           ottl.IgnoreError,
							SpanConditions:      []string{"attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"", "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\""},
							SpanEventConditions: []string{"name != \"test_span_event_name\"", "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\""},
						},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "and-policy-1",
						Type: And,
					},
					AndCfg: AndCfg{
						SubPolicyCfg: []AndSubPolicyCfg{
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:                "test-and-policy-1",
									Type:                NumericAttribute,
									NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:               "test-and-policy-2",
									Type:               StringAttribute,
									StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name: "test-and-policy-3",
									Type: Not,
								},
								NotCfg: NotCfg{
									SubPolicy: NotSubPolicyCfg{
										sharedPolicyCfg: sharedPolicyCfg{
											Name:       "test-and-policy-3-not-sub-policy",
											Type:       Latency,
											LatencyCfg: LatencyCfg{ThresholdMs: 1000},
										},
									},
								},
							},
						},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "not-policy-1",
						Type: Not,
					},
					NotCfg: NotCfg{
						SubPolicy: NotSubPolicyCfg{
							sharedPolicyCfg: sharedPolicyCfg{
								Name:       "test-not-policy-1",
								Type:       Latency,
								LatencyCfg: LatencyCfg{ThresholdMs: 1000},
							},
						},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "composite-policy-1",
						Type: Composite,
					},
					CompositeCfg: CompositeCfg{
						MaxTotalSpansPerSecond: 1000,
						PolicyOrder:            []string{"test-composite-policy-1", "test-composite-policy-2", "test-composite-policy-3"},
						SubPolicyCfg: []CompositeSubPolicyCfg{
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:                "test-composite-policy-1",
									Type:                NumericAttribute,
									NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name:               "test-composite-policy-2",
									Type:               StringAttribute,
									StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
								},
							},
							{
								sharedPolicyCfg: sharedPolicyCfg{
									Name: "test-composite-policy-3",
									Type: AlwaysSample,
								},
							},
						},
						RateAllocation: []RateAllocationCfg{
							{
								Policy:  "test-composite-policy-1",
								Percent: 50,
							},
							{
								Policy:  "test-composite-policy-2",
								Percent: 25,
							},
						},
					},
				},
			},
		}, cfg)
}

func TestConfigValidateNumShards(t *testing.T) {
	tailStorageID := component.MustNewID("tail_storage_pebble")

	testCases := []struct {
		name         string
		numShards    uint32
		tailStorage  *component.ID
		errSubstring string
	}{
		{
			name:      "default is valid",
			numShards: 0,
		},
		{
			name:      "maximum is valid",
			numShards: maxNumShards,
		},
		{
			name:         "above maximum returns error",
			numShards:    maxNumShards + 1,
			errSubstring: "num_shards",
		},
		{
			name:        "single shard with tail storage is valid",
			numShards:   1,
			tailStorage: &tailStorageID,
		},
		{
			name:         "multiple shards with tail storage returns error",
			numShards:    2,
			tailStorage:  &tailStorageID,
			errSubstring: "not supported with tail_storage",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tailStorage != nil {
				prev := tailstorageextension.IsFeatureGateEnabled()
				require.NoError(t, featuregate.GlobalRegistry().Set(tailstorageextension.FeatureGateID, true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(tailstorageextension.FeatureGateID, prev))
				})
			}

			cfg := &Config{
				SamplingStrategy: samplingStrategyTraceComplete,
				NumShards:        tc.numShards,
				TailStorageID:    tc.tailStorage,
			}

			err := cfg.Validate()
			if tc.errSubstring != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstring)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConfigValidateTailStorageFeatureGate(t *testing.T) {
	tailStorageID := component.MustNewID("tail_storage_pebble")

	testCases := []struct {
		name         string
		gateEnabled  bool
		tailStorage  *component.ID
		wantErr      bool
		errSubstring string
	}{
		{
			name:         "tail storage set and gate disabled returns error",
			gateEnabled:  false,
			tailStorage:  &tailStorageID,
			wantErr:      true,
			errSubstring: "'tail_storage' requires",
		},
		{
			name:        "tail storage set and gate enabled is valid",
			gateEnabled: true,
			tailStorage: &tailStorageID,
			wantErr:     false,
		},
		{
			name:        "tail storage not set and gate disabled is valid",
			gateEnabled: false,
			tailStorage: nil,
			wantErr:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prev := tailstorageextension.IsFeatureGateEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(tailstorageextension.FeatureGateID, tc.gateEnabled))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(tailstorageextension.FeatureGateID, prev))
			})

			cfg := &Config{
				SamplingStrategy: samplingStrategyTraceComplete,
				TailStorageID:    tc.tailStorage,
			}

			err := cfg.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstring)
				assert.Contains(t, err.Error(), tailstorageextension.FeatureGateID)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestApplyOTTLErrorModeDefault(t *testing.T) {
	testCases := []struct {
		name          string
		gateEnabled   bool
		errorMode     ottl.ErrorMode
		wantErrorMode ottl.ErrorMode
	}{
		{
			name:          "unset error_mode defaults to propagate when gate disabled",
			gateEnabled:   false,
			errorMode:     "",
			wantErrorMode: ottl.PropagateError,
		},
		{
			name:          "unset error_mode defaults to ignore when gate enabled",
			gateEnabled:   true,
			errorMode:     "",
			wantErrorMode: ottl.IgnoreError,
		},
		{
			name:          "explicit error_mode is preserved when gate enabled",
			gateEnabled:   true,
			errorMode:     ottl.PropagateError,
			wantErrorMode: ottl.PropagateError,
		},
		{
			name:          "explicit error_mode is preserved when gate disabled",
			gateEnabled:   false,
			errorMode:     ottl.IgnoreError,
			wantErrorMode: ottl.IgnoreError,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gateID := "processor.tailsamplingprocessor.defaultErrorModeIgnore"
			prev := telemetry.IsDefaultErrorModeIgnoreEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(gateID, tc.gateEnabled))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(gateID, prev))
			})

			cfg := &Config{
				SamplingStrategy: samplingStrategyTraceComplete,
				PolicyCfgs: []PolicyCfg{
					{
						sharedPolicyCfg: sharedPolicyCfg{
							Name: "test-ottl-policy",
							Type: OTTLCondition,
							OTTLConditionCfg: OTTLConditionCfg{
								ErrorMode:      tc.errorMode,
								SpanConditions: []string{"true"},
							},
						},
					},
				},
			}
			err := cfg.Validate()
			require.NoError(t, err)
			require.Equal(t, tc.wantErrorMode, cfg.PolicyCfgs[0].OTTLConditionCfg.ErrorMode)
		})
	}
}
