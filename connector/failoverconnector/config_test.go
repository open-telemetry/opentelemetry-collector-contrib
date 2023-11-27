// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	testcases := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "default"),
			expected: &Config{
				PipelinePriority: [][]component.ID{
					{
						component.NewIDWithName(component.DataTypeTraces, ""),
					},
				},
				RetryInterval: 10 * time.Minute,
				RetryGap:      30 * time.Second,
				MaxRetries:    10,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				PipelinePriority: [][]component.ID{
					{
						component.NewIDWithName(component.DataTypeTraces, "first"),
						component.NewIDWithName(component.DataTypeTraces, "also_first"),
					},
					{
						component.NewIDWithName(component.DataTypeTraces, "second"),
					},
					{
						component.NewIDWithName(component.DataTypeTraces, "third"),
					},
					{
						component.NewIDWithName(component.DataTypeTraces, "fourth"),
					},
				},
				RetryInterval: 5 * time.Minute,
				RetryGap:      time.Minute,
				MaxRetries:    10,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tc.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	testcases := []struct {
		name string
		id   component.ID
		err  error
	}{
		{
			name: "no priority levels provided",
			id:   component.NewIDWithName(metadata.Type, ""),
			err:  errNoPipelinePriority,
		},
		{
			name: "invalid ratio of retry_gap to retry_interval",
			id:   component.NewIDWithName(metadata.Type, "invalid"),
			err:  errInvalidRetryIntervals,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.id.String(), func(t *testing.T) {
			t.Run(tc.name, func(t *testing.T) {
				cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
				require.NoError(t, err)

				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()

				sub, err := cm.Sub(tc.id.String())
				require.NoError(t, err)
				require.NoError(t, component.UnmarshalConfig(sub, cfg))

				assert.EqualError(t, component.ValidateConfig(cfg), tc.err.Error())
			})
		})
	}
}
