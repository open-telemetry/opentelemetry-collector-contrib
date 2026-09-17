// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	baseCfg := func(rules ...RuleConfig) Config {
		return Config{
			TraceTimeout:  30 * time.Second,
			DecisionDelay: time.Second,
			NumTraces:     100,
			Rules:         rules,
		}
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid_always_sample",
			cfg: baseCfg(RuleConfig{
				Name:    "default",
				Sampler: SamplerConfig{Type: AlwaysSample},
			}),
		},
		{
			name: "valid_probabilistic",
			cfg: baseCfg(RuleConfig{
				Name: "rule1",
				Sampler: SamplerConfig{
					Type:               Probabilistic,
					SamplingPercentage: 10,
				},
			}),
		},
		{
			name: "valid_adaptive_percentage",
			cfg: baseCfg(RuleConfig{
				Name: "rule1",
				Sampler: SamplerConfig{
					Type:                  AdaptivePercentage,
					GoalPercentage:        10,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					Weight:                0.5,
				},
			}),
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
			name: "negative_span_limit",
			cfg: Config{
				TraceTimeout:  time.Second,
				DecisionDelay: time.Second,
				NumTraces:     100,
				SpanLimit:     -1,
				Rules:         []RuleConfig{{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}}},
			},
			wantErr: "span_limit must be non-negative",
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
			cfg: baseCfg(RuleConfig{
				Name:    "r",
				Sampler: SamplerConfig{Type: AlwaysSample},
			}),
		},
		{
			name: "rule_missing_name",
			cfg: baseCfg(RuleConfig{
				Sampler: SamplerConfig{Type: AlwaysSample},
			}),
			wantErr: "name is required",
		},
		{
			name: "reserved_rule_name_prefix",
			cfg: baseCfg(RuleConfig{
				Name:    "_eviction",
				Sampler: SamplerConfig{Type: AlwaysSample},
			}),
			wantErr: `the "_" prefix is reserved`,
		},
		{
			name: "duplicate_rule_name",
			cfg: baseCfg(
				RuleConfig{Name: "a", Sampler: SamplerConfig{Type: AlwaysSample}},
				RuleConfig{Name: "a", Sampler: SamplerConfig{Type: AlwaysSample}},
			),
			wantErr: "duplicate rule name",
		},
		{
			name: "missing_sampler_type",
			cfg: baseCfg(RuleConfig{
				Name: "r",
			}),
			wantErr: "sampler.type is required",
		},
		{
			name: "unknown_sampler_type",
			cfg: baseCfg(RuleConfig{
				Name:    "r",
				Sampler: SamplerConfig{Type: "magic"},
			}),
			wantErr: "unknown sampler.type",
		},
		{
			name: "probabilistic_zero_rate",
			cfg: baseCfg(RuleConfig{
				Name:    "r",
				Sampler: SamplerConfig{Type: Probabilistic},
			}),
			wantErr: "sampling_percentage",
		},
		{
			name: "probabilistic_too_high",
			cfg: baseCfg(RuleConfig{
				Name:    "r",
				Sampler: SamplerConfig{Type: Probabilistic, SamplingPercentage: 150},
			}),
			wantErr: "sampling_percentage",
		},
		{
			name: "adaptive_percentage_missing_fingerprint_attributes",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:           AdaptivePercentage,
					GoalPercentage: 10,
				},
			}),
			wantErr: "fingerprint_attributes",
		},
		{
			name: "adaptive_percentage_invalid_weight",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptivePercentage,
					GoalPercentage:        10,
					FingerprintAttributes: []string{`any.attributes["a"]`},
					Weight:                1.5,
				},
			}),
			wantErr: "weight",
		},
		{
			name: "valid_adaptive_throughput",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					Weight:                0.5,
				},
			}),
		},
		{
			name: "valid_adaptive_throughput_with_initial_sampling_percentage",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                      AdaptiveThroughput,
					GoalThroughput:            100,
					InitialSamplingPercentage: new(25.0),
					FingerprintAttributes:     []string{`resource.attributes["service.name"]`},
					Weight:                    0.5,
				},
			}),
		},
		{
			name: "adaptive_throughput_initial_sampling_percentage_zero_rejected",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                      AdaptiveThroughput,
					GoalThroughput:            100,
					InitialSamplingPercentage: new(0.0),
					FingerprintAttributes:     []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "initial_sampling_percentage must be in (0, 100]",
		},
		{
			name: "adaptive_throughput_initial_sampling_percentage_too_high",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                      AdaptiveThroughput,
					GoalThroughput:            100,
					InitialSamplingPercentage: new(101.0),
					FingerprintAttributes:     []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "initial_sampling_percentage must be in (0, 100]",
		},
		{
			name: "initial_sampling_percentage_rejected_on_adaptive_percentage",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                      AdaptivePercentage,
					GoalPercentage:            10,
					InitialSamplingPercentage: new(25.0),
					FingerprintAttributes:     []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "adaptive_percentage does not use initial_sampling_percentage",
		},
		{
			name: "adaptive_throughput_invalid_weight",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					Weight:                1.0,
				},
			}),
			wantErr: "weight",
		},
		{
			name: "adaptive_throughput_windowed_negative_update_frequency",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             AlgorithmWindowed,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					UpdateFrequency:       -time.Second,
				},
			}),
			wantErr: "update_frequency",
		},
		{
			name: "adaptive_throughput_windowed_negative_lookback_frequency",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             AlgorithmWindowed,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					LookbackFrequency:     -time.Second,
				},
			}),
			wantErr: "lookback_frequency",
		},
		{
			name: "invalid_match_mode",
			cfg: baseCfg(RuleConfig{
				Name:    "r",
				Match:   "some_span",
				Sampler: SamplerConfig{Type: AlwaysSample},
			}),
			wantErr: "match",
		},
		{
			name: "adaptive_throughput_missing_goal",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					FingerprintAttributes: []string{`any.attributes["a"]`},
				},
			}),
			wantErr: "goal_throughput",
		},
		{
			name: "adaptive_throughput_missing_fingerprint_attributes",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:           AdaptiveThroughput,
					GoalThroughput: 100,
				},
			}),
			wantErr: "fingerprint_attributes",
		},
		{
			name: "valid_adaptive_throughput_windowed",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             AlgorithmWindowed,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					UpdateFrequency:       time.Second,
					LookbackFrequency:     30 * time.Second,
				},
			}),
		},
		{
			name: "algorithm_rejected_on_probabilistic",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:               Probabilistic,
					Algorithm:          AlgorithmEMA,
					SamplingPercentage: 10,
				},
			}),
			wantErr: "probabilistic does not use algorithm",
		},
		{
			name: "windowed_rejected_on_adaptive_percentage",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptivePercentage,
					Algorithm:             AlgorithmWindowed,
					GoalPercentage:        10,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "does not support the windowed algorithm",
		},
		{
			name: "unknown_algorithm",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             "sliding",
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "unknown algorithm",
		},
		{
			name: "windowed_rejects_ema_tuning",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             AlgorithmWindowed,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					Weight:                0.5,
				},
			}),
			wantErr: "adaptive_throughput (windowed) does not use weight",
		},
		{
			name: "ema_rejects_windowed_tuning",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					GoalThroughput:        100,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					UpdateFrequency:       time.Second,
				},
			}),
			wantErr: "adaptive_throughput (ema) does not use update_frequency",
		},
		{
			name: "adaptive_throughput_windowed_missing_goal",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptiveThroughput,
					Algorithm:             AlgorithmWindowed,
					FingerprintAttributes: []string{`any.attributes["a"]`},
				},
			}),
			wantErr: "goal_throughput",
		},
		{
			name: "adaptive_throughput_windowed_missing_fingerprint_attributes",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:           AdaptiveThroughput,
					Algorithm:      AlgorithmWindowed,
					GoalThroughput: 100,
				},
			}),
			wantErr: "fingerprint_attributes",
		},
		{
			name: "probabilistic_rejects_fingerprint_attributes_field",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  Probabilistic,
					SamplingPercentage:    10,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
				},
			}),
			wantErr: "probabilistic does not use fingerprint_attributes",
		},
		{
			name: "adaptive_percentage_rejects_windowed_field",
			cfg: baseCfg(RuleConfig{
				Name: "r",
				Sampler: SamplerConfig{
					Type:                  AdaptivePercentage,
					GoalPercentage:        10,
					FingerprintAttributes: []string{`resource.attributes["service.name"]`},
					UpdateFrequency:       time.Second,
				},
			}),
			wantErr: "adaptive_percentage does not use update_frequency",
		},
		{
			name: "eviction_evaluate_default_valid",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{Policy: EvictionEvaluate}
				return c
			}(),
		},
		{
			name: "eviction_probabilistic_valid",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{Policy: EvictionProbabilistic, SamplingPercentage: 10}
				return c
			}(),
		},
		{
			name: "eviction_probabilistic_missing_percentage",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{Policy: EvictionProbabilistic}
				return c
			}(),
			wantErr: "sampling_percentage must be in (0, 100]",
		},
		{
			name: "eviction_percentage_out_of_range",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{Policy: EvictionProbabilistic, SamplingPercentage: 150}
				return c
			}(),
			wantErr: "sampling_percentage must be in (0, 100]",
		},
		{
			name: "eviction_evaluate_rejects_percentage",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{SamplingPercentage: 10}
				return c
			}(),
			wantErr: "sampling_percentage is only used by the probabilistic policy",
		},
		{
			name: "eviction_unknown_policy",
			cfg: func() Config {
				c := baseCfg(RuleConfig{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}})
				c.Eviction = EvictionConfig{Policy: "magic"}
				return c
			}(),
			wantErr: "eviction: policy must be",
		},
		{
			name: "root_span_condition_valid",
			cfg: Config{
				TraceTimeout:      time.Second,
				DecisionDelay:     time.Second,
				NumTraces:         100,
				RootSpanCondition: `IsRootSpan() or span.attributes["hint"] == true`,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
		},
		{
			name: "root_span_condition_invalid_ottl_rejected",
			cfg: Config{
				TraceTimeout:      time.Second,
				DecisionDelay:     time.Second,
				NumTraces:         100,
				RootSpanCondition: `this is not valid ottl`,
				Rules: []RuleConfig{
					{Name: "r", Sampler: SamplerConfig{Type: AlwaysSample}},
				},
			},
			wantErr: "root_span_condition",
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
