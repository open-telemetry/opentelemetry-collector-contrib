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

package k8sclusterreceiver

import (
	"context"
	"testing"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	fakeQuota "github.com/openshift/client-go/quota/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, config.Type("k8s_cluster"), f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	require.Equal(t, &Config{
		ReceiverSettings:           config.NewReceiverSettings(config.NewID(typeStr)),
		Distribution:               distributionKubernetes,
		CollectionInterval:         10 * time.Second,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}, rCfg)

	r, err := f.CreateTracesReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		&config.ReceiverSettings{}, consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Fails with bad K8s Config.
	r, err = f.CreateMetricsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Override for tests.
	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return nil, nil
	}
	r, err = f.CreateMetricsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Test metadata exporters setup.
	ctx := context.Background()
	require.NoError(t, r.Start(ctx, nopHostWithExporters{}))
	require.NoError(t, r.Shutdown(ctx))
	rCfg.MetadataExporters = []string{"nop/withoutmetadata"}
	require.Error(t, r.Start(context.Background(), nopHostWithExporters{}))
}

func TestFactoryDistributions(t *testing.T) {
	f := NewFactory()
	require.Equal(t, config.Type("k8s_cluster"), f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return nil, nil
	}
	rCfg.makeOpenShiftQuotaClient = func(apiConf k8sconfig.APIConfig) (quotaclientset.Interface, error) {
		return fakeQuota.NewSimpleClientset(), nil
	}

	// default
	r, err := f.CreateMetricsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	rr := r.(*kubernetesReceiver)
	require.Nil(t, rr.resourceWatcher.osQuotaClient)

	// openshift
	rCfg.Distribution = "openshift"
	r, err = f.CreateMetricsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)
	rr = r.(*kubernetesReceiver)
	require.NotNil(t, rr.resourceWatcher.osQuotaClient)

	// bad distribution
	rCfg.Distribution = "unknown-distro"
	r, err = f.CreateMetricsReceiver(
		context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rCfg, consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)
	require.EqualError(t, err, "\"unknown-distro\" is not a supported distribution. Must be one of: \"openshift\", \"kubernetes\"")
}

// nopHostWithExporters mocks a receiver.ReceiverHost for test purposes.
type nopHostWithExporters struct {
}

var _ component.Host = (*nopHostWithExporters)(nil)

func (n nopHostWithExporters) ReportFatalError(error) {
}

func (n nopHostWithExporters) GetFactory(component.Kind, config.Type) component.Factory {
	return nil
}

func (n nopHostWithExporters) GetExtensions() map[config.ComponentID]component.Extension {
	return nil
}

func (n nopHostWithExporters) GetExporters() map[config.DataType]map[config.ComponentID]component.Exporter {
	return map[config.DataType]map[config.ComponentID]component.Exporter{
		config.MetricsDataType: {
			config.NewIDWithName("nop", "withoutmetadata"): MockExporter{},
			config.NewIDWithName("nop", "withmetadata"):    mockExporterWithK8sMetadata{},
		},
	}
}
