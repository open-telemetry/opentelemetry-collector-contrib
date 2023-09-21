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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "tail_sampling_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t,
		cfg,
		&Config{
			DecisionWait:            10 * time.Second,
			NumTraces:               100,
			ExpectedNewTracesPerSec: 10,
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
						RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 35},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:         "test-policy-8",
						Type:         SpanCount,
						SpanCountCfg: SpanCountCfg{MinSpans: 2, MaxSpans: 20},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:          "test-policy-9",
						Type:          TraceState,
						TraceStateCfg: TraceStateCfg{Key: "key3", Values: []string{"value1", "value2"}},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:                "test-policy-10",
						Type:                BooleanAttribute,
						BooleanAttributeCfg: BooleanAttributeCfg{Key: "key4", Value: true},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-policy-11",
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
		})
}
