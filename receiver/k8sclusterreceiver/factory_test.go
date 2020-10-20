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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, configmodels.Type("k8s_cluster"), f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	require.Equal(t, &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		CollectionInterval:         10 * time.Second,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}, rCfg)

	r, err := f.CreateTraceReceiver(
		context.Background(), component.ReceiverCreateParams{},
		&configmodels.ReceiverSettings{}, consumertest.NewTracesNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Fails with bad K8s Config.
	r, err = f.CreateMetricsReceiver(
		context.Background(), component.ReceiverCreateParams{},
		rCfg, consumertest.NewMetricsNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Override for tests.
	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return nil, nil
	}
	r, err = f.CreateMetricsReceiver(
		context.Background(), component.ReceiverCreateParams{Logger: zap.NewNop()},
		rCfg, consumertest.NewMetricsNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, r)

	// Test metadata exporters setup.
	ctx := context.Background()
	require.NoError(t, r.Start(ctx, nopHostWithExporters{}))
	require.NoError(t, r.Shutdown(ctx))
	rCfg.MetadataExporters = []string{"exampleexporter/withoutmetadata"}
	require.Error(t, r.Start(context.Background(), nopHostWithExporters{}))
}

// nopHostWithExporters mocks a receiver.ReceiverHost for test purposes.
type nopHostWithExporters struct {
}

var _ component.Host = (*nopHostWithExporters)(nil)

func (n nopHostWithExporters) ReportFatalError(error) {
}

func (n nopHostWithExporters) GetFactory(component.Kind, configmodels.Type) component.Factory {
	return nil
}

func (n nopHostWithExporters) GetExtensions() map[configmodels.Extension]component.ServiceExtension {
	return nil
}

func (n nopHostWithExporters) GetExporters() map[configmodels.DataType]map[configmodels.Exporter]component.Exporter {
	return map[configmodels.DataType]map[configmodels.Exporter]component.Exporter{
		configmodels.MetricsDataType: {
			&configmodels.ExporterSettings{TypeVal: "exampleexporter", NameVal: "exampleexporter/withoutmetadata"}: MockExporter{},
			&configmodels.ExporterSettings{TypeVal: "exampleexporter", NameVal: "exampleexporter/withmetadata"}:    mockExporterWithK8sMetadata{},
		},
	}
}
