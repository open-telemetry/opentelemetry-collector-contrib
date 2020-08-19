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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
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
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}, rCfg)

	r, err := f.CreateTraceReceiver(
		context.Background(), component.ReceiverCreateParams{},
		&configmodels.ReceiverSettings{}, &exportertest.SinkTraceExporter{},
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Fails with bad K8s Config.
	r, err = f.CreateMetricsReceiver(
		context.Background(), component.ReceiverCreateParams{},
		rCfg, &exportertest.SinkMetricsExporter{},
	)
	require.Error(t, err)
	require.Nil(t, r)

	// Override for tests.
	rCfg.makeClient = func(apiConf k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return nil, nil
	}
	r, err = f.CreateMetricsReceiver(
		context.Background(), component.ReceiverCreateParams{},
		rCfg, &exportertest.SinkMetricsExporter{},
	)
	require.NoError(t, err)
	require.NotNil(t, r)
}
