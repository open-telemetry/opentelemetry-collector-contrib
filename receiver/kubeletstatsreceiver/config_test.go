// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeletstatsreceiver

import (
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	duration := 10 * time.Second
	defaultCfg := cfg.Receivers["kubeletstats/default"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/default",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "tls",
			},
		},
		CollectionInterval: duration,
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.ContainerMetricGroup,
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
		},
	}, defaultCfg)

	tlsCfg := cfg.Receivers["kubeletstats/tls"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/tls",
		},
		TCPAddr: confignet.TCPAddr{
			Endpoint: "1.2.3.4:5555",
		},
		ClientConfig: kubelet.ClientConfig{
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
		CollectionInterval: duration,
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.ContainerMetricGroup,
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
		},
	}, tlsCfg)

	saCfg := cfg.Receivers["kubeletstats/sa"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/sa",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "serviceAccount",
			},
			InsecureSkipVerify: true,
		},
		CollectionInterval: duration,
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.ContainerMetricGroup,
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
		},
	}, saCfg)

	metadataCfg := cfg.Receivers["kubeletstats/metadata"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/metadata",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "serviceAccount",
			},
		},
		CollectionInterval: duration,
		ExtraMetadataLabels: []kubelet.MetadataLabel{
			kubelet.MetadataLabelContainerID,
			kubelet.MetadataLabelVolumeType,
		},
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.ContainerMetricGroup,
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
		},
	}, metadataCfg)

	metricGroupsCfg := cfg.Receivers["kubeletstats/metric_groups"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/metric_groups",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "serviceAccount",
			},
		},
		CollectionInterval: 20 * time.Second,
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
			kubelet.VolumeMetricGroup,
		},
	}, metricGroupsCfg)

	metadataWithK8sAPICfg := cfg.Receivers["kubeletstats/metadata_with_k8s_api"].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: "kubeletstats",
			NameVal: "kubeletstats/metadata_with_k8s_api",
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "serviceAccount",
			},
		},
		CollectionInterval: duration,
		ExtraMetadataLabels: []kubelet.MetadataLabel{
			kubelet.MetadataLabelVolumeType,
		},
		MetricGroupsToCollect: []kubelet.MetricGroup{
			kubelet.ContainerMetricGroup,
			kubelet.PodMetricGroup,
			kubelet.NodeMetricGroup,
		},
		K8sAPIConfig: &k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeKubeConfig},
	}, metadataWithK8sAPICfg)
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
		want    *receiverOptions
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
			want: &receiverOptions{
				name: typeStr,
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
				ReceiverSettings: configmodels.ReceiverSettings{
					NameVal: typeStr,
				},
				CollectionInterval:    10 * time.Second,
				ExtraMetadataLabels:   tt.fields.extraMetadataLabels,
				MetricGroupsToCollect: tt.fields.metricGroupsToCollect,
				K8sAPIConfig:          tt.fields.k8sAPIConfig,
			}
			got, err := cfg.getReceiverOptions()
			if (err != nil) != tt.wantErr {
				t.Errorf("getReceiverOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getReceiverOptions() got = %v, want %v", got, tt.want)
			}
		})
	}
}
