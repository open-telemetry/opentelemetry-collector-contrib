// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

// TODO review if tests should succeed on Windows

package kubeletstatsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, "kubeletstats", ft)
}

func TestValidConfig(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateTracesReceiver(t *testing.T) {
	factory := NewFactory()
	traceReceiver, err := factory.CreateTracesReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		factory.CreateDefaultConfig(),
		nil,
	)
	require.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	require.Nil(t, traceReceiver)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	metricsReceiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		tlsConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}

func TestFactoryInvalidExtraMetadataLabels(t *testing.T) {
	factory := NewFactory()
	cfg := Config{
		ExtraMetadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabel("invalid-label")},
	}
	metricsReceiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&cfg,
		consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Equal(t, "label \"invalid-label\" is not supported", err.Error())
	require.Nil(t, metricsReceiver)
}

func TestFactoryBadAuthType(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ClientConfig: kube.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "foo",
			},
		},
	}
	_, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	require.Error(t, err)
}

func TestRestClientErr(t *testing.T) {
	cfg := &Config{
		ClientConfig: kube.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "tls",
			},
		},
	}
	_, err := restClient(zap.NewNop(), cfg)
	require.Error(t, err)
}

func tlsConfig() *Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       time.Second,
		},
		ClientConfig: kube.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: "tls",
			},
			TLSSetting: configtls.TLSSetting{
				CAFile:   "testdata/testcert.crt",
				CertFile: "testdata/testcert.crt",
				KeyFile:  "testdata/testkey.key",
			},
		},
	}
}

func TestCustomUnmarshaller(t *testing.T) {
	type args struct {
		componentParser *confmap.Conf
		intoCfg         *Config
	}
	tests := []struct {
		name                  string
		args                  args
		result                *Config
		mockUnmarshallFailure bool
		configOverride        map[string]interface{}
		wantErr               bool
	}{
		{
			name:    "No config",
			wantErr: false,
		},
		{
			name: "Fail initial unmarshal",
			args: args{
				componentParser: confmap.New(),
			},
			wantErr: true,
		},
		{
			name: "metric_group unset",
			args: args{
				componentParser: confmap.New(),
				intoCfg:         &Config{},
			},
			result: &Config{
				MetricGroupsToCollect: defaultMetricGroups,
			},
		},
		{
			name: "fail to unmarshall metric_groups",
			args: args{
				componentParser: confmap.New(),
				intoCfg:         &Config{},
			},
			mockUnmarshallFailure: true,
			wantErr:               true,
		},
		{
			name: "successfully override metric_group",
			args: args{
				componentParser: confmap.New(),
				intoCfg: &Config{
					ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
						CollectionInterval: 10 * time.Second,
						InitialDelay:       time.Second,
					},
				},
			},
			configOverride: map[string]interface{}{
				"metric_groups":       []string{string(kubelet.ContainerMetricGroup)},
				"collection_interval": 20 * time.Second,
			},
			result: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 20 * time.Second,
					InitialDelay:       time.Second,
				},
				MetricGroupsToCollect: []kubelet.MetricGroup{kubelet.ContainerMetricGroup},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockUnmarshallFailure {
				// some arbitrary failure.
				err := tt.args.componentParser.Merge(confmap.NewFromStringMap(map[string]interface{}{metricGroupsConfig: map[string]string{"foo": "bar"}}))
				require.NoError(t, err)
			}

			// Mock some config overrides.
			if tt.configOverride != nil {
				err := tt.args.componentParser.Merge(confmap.NewFromStringMap(tt.configOverride))
				require.NoError(t, err)
			}

			if err := tt.args.intoCfg.Unmarshal(tt.args.componentParser); (err != nil) != tt.wantErr {
				t.Errorf("customUnmarshaller() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.result != nil {
				assert.Equal(t, tt.result, tt.args.intoCfg)
			}
		})
	}
}
