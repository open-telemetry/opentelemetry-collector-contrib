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
				TraceTimeout:  30 * time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
		},
		{
			name: "valid_deterministic",
			cfg: Config{
				TraceTimeout:  30 * time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  30 * time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
			name:    "missing_trace_timeout",
			cfg:     Config{DecisionDelay: time.Second, NumTraces: 100, Rules: []RuleConfig{{Name: "r"}}},
			wantErr: "trace_timeout",
		},
		{
			name:    "missing_decision_delay",
			cfg:     Config{TraceTimeout: time.Second, NumTraces: 100, Rules: []RuleConfig{{Name: "r"}}},
			wantErr: "decision_delay",
		},
		{
			name:    "missing_num_traces",
			cfg:     Config{TraceTimeout: time.Second, DecisionDelay: time.Second, Rules: []RuleConfig{{Name: "r"}}},
			wantErr: "num_traces",
		},
		{
			name:    "no_rules",
			cfg:     Config{TraceTimeout: time.Second, DecisionDelay: time.Second, NumTraces: 100},
			wantErr: "at least one rule",
		},
		{
			name: "negative_sampled_cache_size",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				DecisionCache: DecisionCacheConfig{SampledCacheSize: -1},
				Rules:         []RuleConfig{{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
			wantErr: "sampled_cache_size",
		},
		{
			name: "negative_non_sampled_cache_size",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				DecisionCache: DecisionCacheConfig{NonSampledCacheSize: -1},
				Rules:         []RuleConfig{{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
			wantErr: "non_sampled_cache_size",
		},
		{
			name: "zero_cache_sizes_allowed",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				DecisionCache: DecisionCacheConfig{SampledCacheSize: 0, NonSampledCacheSize: 0},
				Rules:         []RuleConfig{{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
		},
		{
			name: "rule_missing_name",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules:         []RuleConfig{{Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
			wantErr: "name is required",
		},
		{
			name: "duplicate_rule_name",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules:         []RuleConfig{{Name: "r"}},
			},
			wantErr: "sampler.type is required",
		},
		{
			name: "unknown_sampler_type",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: "magic"}},
				},
			},
			wantErr: "unknown sampler.type",
		},
		{
			name: "deterministic_zero_rate",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: Deterministic}},
				},
			},
			wantErr: "deterministic.sampling_percentage",
		},
		{
			name: "deterministic_too_high",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: Deterministic, Deterministic: DeterministicConfig{SamplingPercentage: 150}}},
				},
			},
			wantErr: "deterministic.sampling_percentage",
		},
		{
			name: "ema_missing_key_fields",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: EMADynamic, EMADynamic: EMADynamicConfig{GoalSamplingPercentage: 10}}},
				},
			},
			wantErr: "key_fields",
		},
		{
			name: "ema_invalid_weight",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  30 * time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  30 * time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
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
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{
						Type:               WindowedThroughput,
						WindowedThroughput: WindowedThroughputConfig{GoalThroughputPerSec: 100},
					}},
				},
			},
			wantErr: "key_fields",
		},
		{
			name: "catch_all_before_later_rule",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
					{Name: "keep-errors", Conditions: []string{"status.code == 2"}, Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
			wantErr: "catch-all rule",
		},
		{
			name: "catch_all_as_last_rule_allowed",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				Rules: []RuleConfig{
					{Name: "keep-errors", Conditions: []string{"status.code == 2"}, Sampler: SamplerConfig{Type: AlwaysSample}},
					{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
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
