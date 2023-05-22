// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr bool
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: func() component.Config {
				cfg := DefaultConfig()
				cfg.ClusterRegion = "us-west-2"
				cfg.JobLabelName = "my_prometheus_job"
				return &cfg
			}(),
		},
		{
			id:       component.NewIDWithName(metadata.Type, "2"),
			expected: exampleConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: func() component.Config {
				cfg := DefaultConfig()
				cfg.DockerLabels = []DockerLabelConfig{
					{
						PortLabel: "IS_NOT_DEFAULT",
					},
				}
				return &cfg
			}(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid"),
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
			if tt.expectedErr {
				assert.Error(t, component.ValidateConfig(cfg))
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		reason string
		cfg    Config
	}{
		{
			reason: "cluster name",
			cfg:    Config{ClusterName: ""},
		},
		{
			reason: "service",
			cfg: Config{
				ClusterName: "c1",
				Services:    []ServiceConfig{{NamePattern: "*"}}, // invalid regex
			},
		},
		{
			reason: "task",
			cfg: Config{
				ClusterName:     "c1",
				TaskDefinitions: []TaskDefinitionConfig{{ArnPattern: "*"}}, // invalid regex
			},
		},
		{
			reason: "docker",
			cfg: Config{
				ClusterName:  "c1",
				DockerLabels: []DockerLabelConfig{{PortLabel: ""}},
			},
		},
	}

	for _, tCase := range cases {
		t.Run(tCase.reason, func(t *testing.T) {
			require.Error(t, tCase.cfg.Validate())
		})
	}
}
