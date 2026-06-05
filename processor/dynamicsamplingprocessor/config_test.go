// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid_always_sample",
			cfg: Config{
				DecisionWait: 30 * time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
		},
		{
			name: "valid_deterministic",
			cfg: Config{
				DecisionWait: 30 * time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{
						Name: "rule1",
						Sampler: SamplerConfig{
							Type:          Deterministic,
							Deterministic: DeterministicConfig{SamplingPercentage: 10},
						},
					},
				},
			},
		},
		{
			name: "valid_ema_dynamic",
			cfg: Config{
				DecisionWait: 30 * time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{
						Name: "rule1",
						Sampler: SamplerConfig{
							Type: EMADynamic,
							EMADynamic: EMADynamicConfig{
								GoalSamplingPercentage: 10,
								KeyFields:              []string{"service.name"},
								Weight:                 0.5,
							},
						},
					},
				},
			},
		},
		{
			name:    "missing_decision_wait",
			cfg:     Config{NumTraces: 100, Rules: []RuleConfig{{Name: "r"}}},
			wantErr: "decision_wait",
		},
		{
			name:    "missing_num_traces",
			cfg:     Config{DecisionWait: time.Second, Rules: []RuleConfig{{Name: "r"}}},
			wantErr: "num_traces",
		},
		{
			name:    "no_rules",
			cfg:     Config{DecisionWait: time.Second, NumTraces: 100},
			wantErr: "at least one rule",
		},
		{
			name: "rule_missing_name",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules:        []RuleConfig{{Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
			wantErr: "name is required",
		},
		{
			name: "duplicate_rule_name",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "a", Sampler: SamplerConfig{Type: AlwaysSample}},
					{Name: "a", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
			wantErr: "duplicate rule name",
		},
		{
			name: "missing_sampler_type",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules:        []RuleConfig{{Name: "r"}},
			},
			wantErr: "sampler.type is required",
		},
		{
			name: "unknown_sampler_type",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: "magic"}},
				},
			},
			wantErr: "unknown sampler.type",
		},
		{
			name: "deterministic_zero_rate",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: Deterministic}},
				},
			},
			wantErr: "deterministic.sampling_percentage",
		},
		{
			name: "deterministic_too_high",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: Deterministic, Deterministic: DeterministicConfig{SamplingPercentage: 150}}},
				},
			},
			wantErr: "deterministic.sampling_percentage",
		},
		{
			name: "ema_missing_key_fields",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: EMADynamic, EMADynamic: EMADynamicConfig{GoalSamplingPercentage: 10}}},
				},
			},
			wantErr: "key_fields",
		},
		{
			name: "ema_invalid_weight",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type: EMADynamic,
						EMADynamic: EMADynamicConfig{
							GoalSamplingPercentage: 10,
							KeyFields:              []string{"a"},
							Weight:                 1.5,
						},
					}},
				},
			},
			wantErr: "weight",
		},
		{
			name: "valid_ema_throughput",
			cfg: Config{
				DecisionWait: 30 * time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{
						Name: "r",
						Sampler: SamplerConfig{
							Type: EMAThroughput,
							EMAThroughput: EMAThroughputConfig{
								GoalThroughputPerSec: 100,
								KeyFields:            []string{"service.name"},
								Weight:               0.5,
							},
						},
					},
				},
			},
		},
		{
			name: "ema_throughput_missing_goal",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type:          EMAThroughput,
						EMAThroughput: EMAThroughputConfig{KeyFields: []string{"a"}},
					}},
				},
			},
			wantErr: "goal_throughput_per_sec",
		},
		{
			name: "ema_throughput_missing_key_fields",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type:          EMAThroughput,
						EMAThroughput: EMAThroughputConfig{GoalThroughputPerSec: 100},
					}},
				},
			},
			wantErr: "key_fields",
		},
		{
			name: "valid_windowed_throughput",
			cfg: Config{
				DecisionWait: 30 * time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{
						Name: "r",
						Sampler: SamplerConfig{
							Type: WindowedThroughput,
							WindowedThroughput: WindowedThroughputConfig{
								GoalThroughputPerSec: 100,
								KeyFields:            []string{"service.name"},
								UpdateFrequency:      time.Second,
								LookbackFrequency:    30 * time.Second,
							},
						},
					},
				},
			},
		},
		{
			name: "windowed_throughput_missing_goal",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type:               WindowedThroughput,
						WindowedThroughput: WindowedThroughputConfig{KeyFields: []string{"a"}},
					}},
				},
			},
			wantErr: "goal_throughput_per_sec",
		},
		{
			name: "windowed_throughput_missing_key_fields",
			cfg: Config{
				DecisionWait: time.Second,
				NumTraces:    100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type:               WindowedThroughput,
						WindowedThroughput: WindowedThroughputConfig{GoalThroughputPerSec: 100},
					}},
				},
			},
			wantErr: "key_fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
