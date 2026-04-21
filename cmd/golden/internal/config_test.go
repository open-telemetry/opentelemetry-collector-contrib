// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_ReadConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		cfg  *Config
		err  string
	}{
		{
			name: "expected",
			args: []string{
				"--expected", "foo.yaml",
			},
			cfg: &Config{ExpectedFile: "foo.yaml", CompareOptions: []pmetrictest.CompareMetricsOption{}},
		},
		{
			name: "ignore start timestamp",
			args: []string{
				"--expected", "foo.yaml",
				"--ignore-start-timestamp",
			},
			cfg: &Config{
				ExpectedFile:   "foo.yaml",
				CompareOptions: []pmetrictest.CompareMetricsOption{pmetrictest.IgnoreStartTimestamp()},
			},
		},
		{
			name: "write expected",
			args: []string{
				"--expected", "foo.yaml",
				"--write-expected",
			},
			cfg: &Config{
				ExpectedFile:   "foo.yaml",
				WriteExpected:  true,
				CompareOptions: []pmetrictest.CompareMetricsOption{},
			},
		},
		{
			name: "missing resource attribute key",
			args: []string{
				"--expected", "foo.yaml",
				"--ignore-resource-attribute-value",
			},
			err: "--ignore-resource-attribute-value requires an argument",
		},
		{
			name: "missing argument metric key",
			args: []string{
				"--expected", "foo.yaml",
				"--ignore-metric-attribute-value",
			},
			err: "--ignore-metric-attribute-value requires an argument",
		},
		{
			name: "ignore metric values",
			args: []string{
				"--expected", "foo.yaml",
				"--ignore-metric-values",
			},
			cfg: &Config{
				ExpectedFile:   "foo.yaml",
				CompareOptions: []pmetrictest.CompareMetricsOption{pmetrictest.IgnoreMetricValues()},
			},
		},
		{
			name: "ignore metric values for specific metrics",
			args: []string{
				"--expected", "foo.yaml",
				"--ignore-metric-values",
				"foo.bar",
			},
			cfg: &Config{
				ExpectedFile:   "foo.yaml",
				CompareOptions: []pmetrictest.CompareMetricsOption{pmetrictest.IgnoreMetricValues("foo.bar")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			cfg, err := ReadConfig(test.args)
			if test.err != "" {
				require.EqualError(tt, err, test.err)
			} else {
				assert.Equal(tt, test.cfg.WriteExpected, cfg.WriteExpected)
				assert.Equal(tt, test.cfg.ExpectedFile, cfg.ExpectedFile)
				assert.Len(tt, test.cfg.CompareOptions, len(cfg.CompareOptions))
			}
		})
	}
}
