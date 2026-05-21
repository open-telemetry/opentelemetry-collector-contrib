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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"
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

func TestConfigMarshalPoliciesRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "baseline",
					Type: Probabilistic,
					ProbabilisticCfg: ProbabilisticCfg{
						HashSalt:           "salt",
						SamplingPercentage: 1,
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "errors",
					Type: StatusCode,
					StatusCodeCfg: StatusCodeCfg{
						StatusCodes: []string{"ERROR"},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "fanout",
					Type: And,
				},
				AndCfg: AndCfg{
					SubPolicyCfg: []AndSubPolicyCfg{
						{
							sharedPolicyCfg: sharedPolicyCfg{
								Name: "http-only",
								Type: StringAttribute,
								StringAttributeCfg: StringAttributeCfg{
									Key:    "service.name",
									Values: []string{"api"},
								},
							},
						},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "not-latency",
					Type: Not,
				},
				NotCfg: NotCfg{
					SubPolicy: NotSubPolicyCfg{
						sharedPolicyCfg: sharedPolicyCfg{
							Name: "slow",
							Type: Latency,
							LatencyCfg: LatencyCfg{
								ThresholdMs: 5000,
							},
						},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "composite",
					Type: Composite,
				},
				CompositeCfg: CompositeCfg{
					MaxTotalSpansPerSecond: 100,
					PolicyOrder:            []string{"nested-and", "nested-prob"},
					SubPolicyCfg: []CompositeSubPolicyCfg{
						{
							sharedPolicyCfg: sharedPolicyCfg{
								Name: "nested-and",
								Type: And,
							},
							AndCfg: AndCfg{
								SubPolicyCfg: []AndSubPolicyCfg{
									{
										sharedPolicyCfg: sharedPolicyCfg{
											Name: "nested-status",
											Type: StatusCode,
											StatusCodeCfg: StatusCodeCfg{
												StatusCodes: []string{"ERROR", "UNSET"},
											},
										},
									},
								},
							},
						},
						{
							sharedPolicyCfg: sharedPolicyCfg{
								Name: "nested-prob",
								Type: Probabilistic,
								ProbabilisticCfg: ProbabilisticCfg{
									SamplingPercentage: 10,
								},
							},
						},
					},
					RateAllocation: []RateAllocationCfg{
						{
							Policy:  "nested-prob",
							Percent: 25,
						},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "drop-debug",
					Type: Drop,
				},
				DropCfg: DropCfg{
					SubPolicyCfg: []AndSubPolicyCfg{
						{
							sharedPolicyCfg: sharedPolicyCfg{
								Name: "debug-attr",
								Type: BooleanAttribute,
								BooleanAttributeCfg: BooleanAttributeCfg{
									Key:   "debug",
									Value: true,
								},
							},
						},
					},
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "extension-policy",
					Type: PolicyType("custom_extension"),
					ExtensionCfg: map[string]any{
						"custom_extension": map[string]any{"mode": "keep"},
					},
				},
			},
		},
	}

	cm := confmap.New()
	require.NoError(t, cm.Marshal(cfg))

	policies, ok := cm.ToStringMap()["policies"].([]any)
	require.True(t, ok)
	require.Len(t, policies, 7)

	baseline := policies[0].(map[string]any)
	assert.Equal(t, "baseline", baseline["name"])
	assert.Equal(t, Probabilistic, baseline["type"])
	assert.Equal(t, map[string]any{
		"hash_salt":           "salt",
		"sampling_percentage": 1.0,
	}, baseline["probabilistic"])

	errors := policies[1].(map[string]any)
	assert.Equal(t, "errors", errors["name"])
	assert.Equal(t, StatusCode, errors["type"])
	assert.Equal(t, map[string]any{
		"status_codes": []any{"ERROR"},
	}, errors["status_code"])

	fanout := policies[2].(map[string]any)
	assert.Equal(t, "fanout", fanout["name"])
	assert.Equal(t, And, fanout["type"])
	fanoutAnd := fanout["and"].(map[string]any)
	fanoutSub := fanoutAnd["and_sub_policy"].([]any)[0].(map[string]any)
	assert.Equal(t, "http-only", fanoutSub["name"])
	assert.Equal(t, StringAttribute, fanoutSub["type"])
	assert.Equal(t, map[string]any{
		"cache_max_size":         0,
		"enabled_regex_matching": false,
		"invert_match":           false,
		"key":                    "service.name",
		"values":                 []any{"api"},
	}, fanoutSub["string_attribute"])

	notLatency := policies[3].(map[string]any)
	assert.Equal(t, "not-latency", notLatency["name"])
	assert.Equal(t, Not, notLatency["type"])
	notSub := notLatency["not"].(map[string]any)["not_sub_policy"].(map[string]any)
	assert.Equal(t, "slow", notSub["name"])
	assert.Equal(t, Latency, notSub["type"])
	assert.Equal(t, map[string]any{
		"threshold_ms":       int64(5000),
		"upper_threshold_ms": int64(0),
	}, notSub["latency"])

	composite := policies[4].(map[string]any)
	assert.Equal(t, "composite", composite["name"])
	assert.Equal(t, Composite, composite["type"])
	compositeCfg := composite["composite"].(map[string]any)
	assert.Equal(t, int64(100), compositeCfg["max_total_spans_per_second"])
	assert.Equal(t, []any{"nested-and", "nested-prob"}, compositeCfg["policy_order"])
	compositePolicies := compositeCfg["composite_sub_policy"].([]any)
	require.Len(t, compositePolicies, 2)
	nestedAnd := compositePolicies[0].(map[string]any)
	assert.Equal(t, "nested-and", nestedAnd["name"])
	assert.Equal(t, And, nestedAnd["type"])
	nestedStatus := nestedAnd["and"].(map[string]any)["and_sub_policy"].([]any)[0].(map[string]any)
	assert.Equal(t, "nested-status", nestedStatus["name"])
	assert.Equal(t, StatusCode, nestedStatus["type"])
	assert.Equal(t, map[string]any{
		"status_codes": []any{"ERROR", "UNSET"},
	}, nestedStatus["status_code"])
	nestedProb := compositePolicies[1].(map[string]any)
	assert.Equal(t, "nested-prob", nestedProb["name"])
	assert.Equal(t, Probabilistic, nestedProb["type"])
	assert.Equal(t, map[string]any{
		"hash_salt":           "",
		"sampling_percentage": 10.0,
	}, nestedProb["probabilistic"])

	dropDebug := policies[5].(map[string]any)
	assert.Equal(t, "drop-debug", dropDebug["name"])
	assert.Equal(t, Drop, dropDebug["type"])
	dropSub := dropDebug["drop"].(map[string]any)["drop_sub_policy"].([]any)[0].(map[string]any)
	assert.Equal(t, "debug-attr", dropSub["name"])
	assert.Equal(t, BooleanAttribute, dropSub["type"])
	assert.Equal(t, map[string]any{
		"invert_match": false,
		"key":          "debug",
		"value":        true,
	}, dropSub["boolean_attribute"])

	extensionPolicy := policies[6].(map[string]any)
	assert.Equal(t, "extension-policy", extensionPolicy["name"])
	assert.Equal(t, PolicyType("custom_extension"), extensionPolicy["type"])
	assert.Equal(t, map[string]any{"mode": "keep"}, extensionPolicy["custom_extension"])
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
