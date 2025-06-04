// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
)

func TestConfig(t *testing.T) {
	for _, tc := range []struct {
		path      string // relative to `testdata/configs` directory
		expected  *Config
		errorMsgs []string // all error message are checked
	}{
		{
			path:      "empty",
			errorMsgs: []string{"no configuration provided"},
		},
		{
			path: "without_name",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "missing required metric name"),
				fullErrorForSignal(t, "datapoints", "missing required metric name"),
				fullErrorForSignal(t, "logs", "missing required metric name"),
				fullErrorForSignal(t, "profiles", "missing required metric name"),
			},
		},
		{
			path: "no_key_attributes",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "attributes validation failed"),
				fullErrorForSignal(t, "datapoints", "attributes validation failed"),
				fullErrorForSignal(t, "logs", "attributes validation failed"),
				fullErrorForSignal(t, "profiles", "attributes validation failed"),
			},
		},
		{
			path: "duplicate_attributes",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "attributes validation failed"),
				fullErrorForSignal(t, "datapoints", "attributes validation failed"),
				fullErrorForSignal(t, "logs", "attributes validation failed"),
				fullErrorForSignal(t, "profiles", "attributes validation failed"),
			},
		},
		{
			path: "with_optional_and_default_value",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "attributes validation failed: only one of default_value or optional should be set"),
				fullErrorForSignal(t, "datapoints", "attributes validation failed: only one of default_value or optional should be set"),
				fullErrorForSignal(t, "logs", "attributes validation failed: only one of default_value or optional should be set"),
				fullErrorForSignal(t, "profiles", "attributes validation failed: only one of default_value or optional should be set"),
			},
		},
		{
			path: "invalid_histogram",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "histogram validation failed"),
				fullErrorForSignal(t, "datapoints", "histogram validation failed"),
				fullErrorForSignal(t, "logs", "histogram validation failed"),
				fullErrorForSignal(t, "profiles", "histogram validation failed"),
			},
		},
		{
			path: "invalid_exponential_histogram",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "histogram validation failed"),
				fullErrorForSignal(t, "datapoints", "histogram validation failed"),
				fullErrorForSignal(t, "logs", "histogram validation failed"),
				fullErrorForSignal(t, "profiles", "histogram validation failed"),
			},
		},
		{
			path: "invalid_sum",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "sum validation failed"),
				fullErrorForSignal(t, "datapoints", "sum validation failed"),
				fullErrorForSignal(t, "logs", "sum validation failed"),
				fullErrorForSignal(t, "profiles", "sum validation failed"),
			},
		},
		{
			path: "multiple_metric",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "exactly one of the metrics must be defined"),
				fullErrorForSignal(t, "datapoints", "exactly one of the metrics must be defined"),
				fullErrorForSignal(t, "logs", "exactly one of the metrics must be defined"),
				fullErrorForSignal(t, "profiles", "exactly one of the metrics must be defined"),
			},
		},
		{
			path: "invalid_grok_type_map",
			errorMsgs: []string{
				fullErrorForSignal(t, "logs", "ExtractGrokPatterns: a single key selector[key] is required for signal to gauge"),
			},
		},
		{
			path: "invalid_ottl_value_expression",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "failed to parse value OTTL expression"),
				fullErrorForSignal(t, "datapoints", "failed to parse value OTTL expression"),
				fullErrorForSignal(t, "logs", "failed to parse value OTTL expression"),
				fullErrorForSignal(t, "profiles", "failed to parse value OTTL expression"),
			},
		},
		{
			path: "invalid_ottl_conditions",
			errorMsgs: []string{
				fullErrorForSignal(t, "spans", "failed to parse OTTL conditions"),
				fullErrorForSignal(t, "datapoints", "failed to parse OTTL conditions"),
				fullErrorForSignal(t, "logs", "failed to parse OTTL conditions"),
				fullErrorForSignal(t, "profiles", "failed to parse OTTL conditions"),
			},
		},
		{
			path: "valid_full",
			expected: &Config{
				Spans: []MetricInfo{
					{
						Name:                      "span.exp_histogram",
						Description:               "Exponential histogram",
						Unit:                      "us",
						IncludeResourceAttributes: []Attribute{{Key: "key.1", DefaultValue: "foo"}},
						Attributes: []Attribute{
							{Key: "key.2", DefaultValue: "bar"},
							{Key: "key.3", Optional: true},
						},
						Conditions: []string{
							`attributes["some.optional.1"] != nil`,
							`resource.attributes["some.optional.2"] != nil`,
						},
						ExponentialHistogram: &ExponentialHistogram{
							MaxSize: 10,
							Count:   "1",
							Value:   "Microseconds(end_time - start_time)",
						},
					},
					{
						Name:                      "span.histogram",
						Description:               "Histogram",
						Unit:                      "us",
						IncludeResourceAttributes: []Attribute{{Key: "key.1", DefaultValue: "foo"}},
						Attributes: []Attribute{
							{Key: "key.2", DefaultValue: "bar"},
							{Key: "key.3", Optional: true},
						},
						Conditions: []string{
							`attributes["some.optional.1"] != nil`,
							`resource.attributes["some.optional.2"] != nil`,
						},
						Histogram: &Histogram{
							Buckets: []float64{1.1, 11.1, 111.1},
							Count:   "1",
							Value:   "Microseconds(end_time - start_time)",
						},
					},
				},
				Datapoints: []MetricInfo{
					{
						Name:                      "dp.sum",
						Description:               "Sum",
						Unit:                      "ms",
						IncludeResourceAttributes: []Attribute{{Key: "key.1", DefaultValue: "foo"}},
						Attributes: []Attribute{
							{Key: "key.2", DefaultValue: "bar"},
							{Key: "key.3", Optional: true},
						},
						Conditions: []string{
							`attributes["some.optional.1"] != nil`,
							`IsDouble(attributes["some.optional.1"])`,
						},
						Sum: &Sum{
							Value: `attributes["some.optional.1"]`,
						},
					},
				},
				Logs: []MetricInfo{
					{
						Name:                      "log.sum",
						Description:               "Sum",
						Unit:                      "1",
						IncludeResourceAttributes: []Attribute{{Key: "key.1", DefaultValue: "foo"}},
						Attributes: []Attribute{
							{Key: "key.2", DefaultValue: "bar"},
							{Key: "key.3", Optional: true},
						},
						Conditions: []string{
							`attributes["some.optional.1"] != nil`,
						},
						Sum: &Sum{
							Value: "1",
						},
					},
				},
				Profiles: []MetricInfo{
					{
						Name:                      "profile.sum",
						Description:               "Sum",
						Unit:                      "1",
						IncludeResourceAttributes: []Attribute{{Key: "key.1", DefaultValue: "foo"}},
						Attributes: []Attribute{
							{Key: "key.2", DefaultValue: "bar"},
							{Key: "key.3", Optional: true},
						},
						Conditions: []string{
							`duration_unix_nano > 0`,
						},
						Sum: &Sum{
							Value: "1",
						},
					},
				},
			},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := filepath.Join("..", "testdata", "configs")
			cfg := &Config{}
			cm, err := confmaptest.LoadConf(filepath.Join(dir, tc.path+".yaml"))
			require.NoError(t, err)

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(&cfg))

			err = xconfmap.Validate(cfg)
			if len(tc.errorMsgs) > 0 {
				for _, errMsg := range tc.errorMsgs {
					assert.ErrorContains(t, err, errMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

const validationMsgFormat = "failed to validate %s configuration: %s"

func fullErrorForSignal(t *testing.T, signal, errMsg string) string {
	t.Helper()

	switch signal {
	case "spans", "datapoints", "logs", "profiles":
		return fmt.Sprintf(validationMsgFormat, signal, errMsg)
	default:
		panic("unhandled signal type")
	}
}
