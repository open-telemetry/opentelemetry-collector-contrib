// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	duration := 10 * time.Second

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, "default"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "tls",
					},
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tls"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: duration,
				},
				TCPAddr: confignet.TCPAddr{
					Endpoint: "1.2.3.4:5555",
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "tls",
					},
					TLSSetting: configtls.TLSSetting{
						CAFile:   "/path/to/ca.crt",
						CertFile: "/path/to/apiserver.crt",
						KeyFile:  "/path/to/apiserver.key",
					},
					InsecureSkipVerify: true,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "sa"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
					InsecureSkipVerify: true,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metadata"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				ExtraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
					kubelet.MetadataLabelVolumeType,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metric_groups"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 20 * time.Second,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
					kubelet.VolumeMetricGroup,
				},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metadata_with_k8s_api"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: duration,
				},
				ClientConfig: kube.ClientConfig{
					APIConfig: k8sconfig.APIConfig{
						AuthType: "serviceAccount",
					},
				},
				ExtraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelVolumeType,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.ContainerMetricGroup,
					kubelet.PodMetricGroup,
					kubelet.NodeMetricGroup,
				},
				K8sAPIConfig:         &k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestGetReceiverOptions(t *testing.T) {
	type fields struct {
		extraMetadataLabels   []kubelet.MetadataLabel
		metricGroupsToCollect []kubelet.MetricGroup
		k8sAPIConfig          *k8sconfig.APIConfig
	}
	tests := []struct {
		name    string
		fields  fields
		want    *scraperOptions
		wantErr bool
	}{
		{
			name: "Valid config",
			fields: fields{
				extraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
				},
				metricGroupsToCollect: []kubelet.MetricGroup{
					kubelet.NodeMetricGroup,
					kubelet.PodMetricGroup,
				},
			},
			want: &scraperOptions{
				extraMetadataLabels: []kubelet.MetadataLabel{
					kubelet.MetadataLabelContainerID,
				},
				metricGroupsToCollect: map[kubelet.MetricGroup]bool{
					kubelet.NodeMetricGroup: true,
					kubelet.PodMetricGroup:  true,
				},
				collectionInterval: 10 * time.Second,
			},
		},
		{
			name: "Invalid metric group",
			fields: fields{
				extraMetadataLabels: []kubelet.MetadataLabel{
					"unsupported",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid extra metadata",
			fields: fields{
				metricGroupsToCollect: []kubelet.MetricGroup{
					"unsupported",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Fails to create k8s API client",
			fields: fields{
				k8sAPIConfig: &k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeServiceAccount},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 10 * time.Second,
				},
				ExtraMetadataLabels:   tt.fields.extraMetadataLabels,
				MetricGroupsToCollect: tt.fields.metricGroupsToCollect,
				K8sAPIConfig:          tt.fields.k8sAPIConfig,
			}
			got, err := cfg.getReceiverOptions()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
