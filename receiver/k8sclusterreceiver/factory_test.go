// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver

import (
	"context"
	"testing"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	fakeQuota "github.com/openshift/client-go/quota/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, metadata.Type, f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	require.Equal(t, &Config{
		Distribution:               distributionKubernetes,
		CollectionInterval:         10 * time.Second,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
		MetadataCollectionInterval: 5 * time.Minute,
		MetricsBuilderConfig:       metadata.DefaultMetricsBuilderConfig(),
	}, rCfg)

	r, err := f.CreateTraces(
		context.Background(), receivertest.NewNopSettings(),
		cfg, consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, r)

	r = newTestReceiver(t, rCfg)

	// Test metadata exporters setup.
	ctx := context.Background()
	require.NoError(t, r.Start(ctx, newNopHostWithExporters()))
	require.NoError(t, r.Shutdown(ctx))

	rCfg.MetadataExporters = []string{"nop/withoutmetadata"}
	r = newTestReceiver(t, rCfg)
	require.Error(t, r.Start(context.Background(), newNopHostWithExporters()))
}

func TestFactoryDistributions(t *testing.T) {
	f := NewFactory()
	require.Equal(t, metadata.Type, f.Type())

	cfg := f.CreateDefaultConfig()
	rCfg, ok := cfg.(*Config)
	require.True(t, ok)

	// default
	r := newTestReceiver(t, rCfg)
	err := r.Start(context.Background(), newNopHost())
	require.NoError(t, err)
	require.Nil(t, r.resourceWatcher.osQuotaClient)

	// openshift
	rCfg.Distribution = "openshift"
	r = newTestReceiver(t, rCfg)
	err = r.Start(context.Background(), newNopHost())
	require.NoError(t, err)
	require.NotNil(t, r.resourceWatcher.osQuotaClient)
}

func newTestReceiver(t *testing.T, cfg *Config) *kubernetesReceiver {
	r, err := newReceiver(context.Background(), receivertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, r)
	rcvr, ok := r.(*kubernetesReceiver)
	require.True(t, ok)
	rcvr.resourceWatcher.makeClient = func(_ k8sconfig.APIConfig) (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	rcvr.resourceWatcher.makeOpenShiftQuotaClient = func(_ k8sconfig.APIConfig) (quotaclientset.Interface, error) {
		return fakeQuota.NewSimpleClientset(), nil
	}
	return rcvr
}

// nopHostWithExporters mocks a receiver.ReceiverHost for test purposes.
type nopHostWithExporters struct {
	component.Host
}

func newNopHostWithExporters() component.Host {
	return &nopHostWithExporters{Host: newNopHost()}
}

func (n *nopHostWithExporters) GetExporters() map[pipeline.Signal]map[component.ID]component.Component {
	return map[pipeline.Signal]map[component.ID]component.Component{
		pipeline.SignalMetrics: {
			component.MustNewIDWithName("nop", "withoutmetadata"): MockExporter{},
			component.MustNewIDWithName("nop", "withmetadata"):    mockExporterWithK8sMetadata{},
		},
	}
}

func TestNewSharedReceiver(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()

	mc := consumertest.NewNop()
	mr, err := newMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, mc)
	require.NoError(t, err)

	// Verify that the metric consumer is correctly set.
	kr := mr.(*sharedcomponent.SharedComponent).Unwrap().(*kubernetesReceiver)
	assert.Equal(t, mc, kr.metricsConsumer)

	lc := consumertest.NewNop()
	lr, err := newLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, lc)
	require.NoError(t, err)

	// Verify that the log consumer is correct set.
	kr = lr.(*sharedcomponent.SharedComponent).Unwrap().(*kubernetesReceiver)
	assert.Equal(t, lc, kr.resourceWatcher.entityLogConsumer)

	// Make sure only one receiver is created both for metrics and logs.
	assert.Equal(t, mr, lr)
}
