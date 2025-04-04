// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8slogreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "default"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "ds-stdout"),
			expected: &Config{
				Discovery: SourceConfig{
					K8sAPI:      k8sconfig.APIConfig{AuthType: "serviceAccount"},
					Mode:        ModeDaemonSetStdout,
					NodeFromEnv: DefaultNodeFromEnv,
					HostRoot:    "/host_root",
					RuntimeAPIs: []RuntimeAPIConfig{
						{
							&DockerConfig{
								baseRuntimeAPIConfig: baseRuntimeAPIConfig{
									Type: "docker",
								},
								Addr:           "unix:///host_root/var/run/docker.sock",
								ContainerdAddr: "unix:///host_root/run/containerd/containerd.sock",
							},
						},
						{
							&CRIConfig{
								baseRuntimeAPIConfig: baseRuntimeAPIConfig{
									Type: "cri",
								},
								Addr:            "unix:///host_root/run/containerd/containerd.sock",
								ContainerdState: "/host_root/run/containerd",
							},
						},
					},
					Filter: []FilterConfig{
						{
							Annotations: []MapFilterConfig{
								{
									Op:  "exists",
									Key: "io.opentelemetry.collectlog",
								},
							},
							Namespaces: []ValueFilterConfig{
								{
									Op:    "not-equals",
									Value: "kube-system",
								},
							},
						},
					},
				},
				Extract: ExtractConfig{
					Metadata: []string{
						"k8s.pod.name",
						"k8s.pod.uid",
						"k8s.container.name",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
