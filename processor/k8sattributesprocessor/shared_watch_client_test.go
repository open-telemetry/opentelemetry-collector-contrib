// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

type countingClient struct {
	*fakeClient

	mu         sync.Mutex
	startCount int
	stopCount  int
}

func (c *countingClient) Start() error {
	c.mu.Lock()
	c.startCount++
	c.mu.Unlock()
	return c.fakeClient.Start()
}

func (c *countingClient) Stop() {
	c.mu.Lock()
	c.stopCount++
	c.mu.Unlock()
	c.fakeClient.Stop()
}

func (c *countingClient) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.startCount, c.stopCount
}

type countingClientProvider struct {
	mu      sync.Mutex
	clients []*countingClient
}

func (p *countingClientProvider) provider(
	set component.TelemetrySettings,
	apiConfig k8sconfig.APIConfig,
	rules kube.ExtractionRules,
	filters kube.Filters,
	associations []kube.Association,
	excludes kube.Excludes,
	apiClientProvider kube.APIClientsetProvider,
	informers kube.InformersFactoryList,
	waitForMetadata bool,
	waitForMetadataTimeout time.Duration,
) (kube.Client, error) {
	client, err := newFakeClient(
		set,
		apiConfig,
		rules,
		filters,
		associations,
		excludes,
		apiClientProvider,
		informers,
		waitForMetadata,
		waitForMetadataTimeout,
	)
	if err != nil {
		return nil, err
	}

	countingClient := &countingClient{fakeClient: client.(*fakeClient)}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients = append(p.clients, countingClient)
	return countingClient, nil
}

func (p *countingClientProvider) snapshot() []*countingClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]*countingClient(nil), p.clients...)
}

func setShareInformerCachesFeatureGate(t *testing.T, enabled bool) {
	t.Helper()

	previousValue := metadata.ProcessorK8sattributesShareInformerCachesFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesShareInformerCachesFeatureGate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.ProcessorK8sattributesShareInformerCachesFeatureGate.ID(), previousValue))
	})
}

func sharedWatchClientCount(t *testing.T, factory *kubernetesProcessorFactory) int {
	t.Helper()

	comps := reflect.ValueOf(factory.sharedWatchClients).Elem().FieldByName("comps")
	require.True(t, comps.IsValid())
	return comps.Len()
}

func createAllSignalProcessors(
	t *testing.T,
	factory *kubernetesProcessorFactory,
	cfg component.Config,
	options ...option,
) (processor.Traces, processor.Metrics, processor.Logs, xprocessor.Profiles, *kubernetesprocessor, *kubernetesprocessor, *kubernetesprocessor, *kubernetesprocessor) {
	t.Helper()

	var traceKP, metricsKP, logsKP, profilesKP *kubernetesprocessor
	tp, err := newTracesProcessorWithFactory(factory, cfg, consumertest.NewNop(), append(options, withExtractKubernetesProcessorInto(&traceKP))...)
	require.NoError(t, err)
	mp, err := newMetricsProcessorWithFactory(factory, cfg, consumertest.NewNop(), append(options, withExtractKubernetesProcessorInto(&metricsKP))...)
	require.NoError(t, err)
	lp, err := newLogsProcessorWithFactory(factory, cfg, consumertest.NewNop(), append(options, withExtractKubernetesProcessorInto(&logsKP))...)
	require.NoError(t, err)
	pp, err := newProfilesProcessorWithFactory(factory, cfg, consumertest.NewNop(), append(options, withExtractKubernetesProcessorInto(&profilesKP))...)
	require.NoError(t, err)

	return tp, mp, lp, pp, traceKP, metricsKP, logsKP, profilesKP
}

func startAllProcessors(t *testing.T, processors ...component.Component) {
	t.Helper()

	for _, proc := range processors {
		require.NoError(t, proc.Start(t.Context(), componenttest.NewNopHost()))
	}
}

func shutdownAllProcessors(t *testing.T, processors ...component.Component) {
	t.Helper()

	for _, proc := range processors {
		require.NoError(t, proc.Shutdown(t.Context()))
	}
}

func TestSharedWatchClientReusedAcrossSignals(t *testing.T) {
	setShareInformerCachesFeatureGate(t, true)

	factory := newTestFactory()
	clientProvider := &countingClientProvider{}
	cfg := NewFactory().CreateDefaultConfig()

	tp, mp, lp, pp, traceKP, metricsKP, logsKP, profilesKP := createAllSignalProcessors(
		t,
		factory,
		cfg,
		withKubeClientProvider(clientProvider.provider),
	)
	startAllProcessors(t, tp, mp, lp, pp)

	require.NotNil(t, traceKP.sharedKubeComponent)
	require.Same(t, traceKP.sharedKubeComponent, metricsKP.sharedKubeComponent)
	require.Same(t, traceKP.sharedKubeComponent, logsKP.sharedKubeComponent)
	require.Same(t, traceKP.sharedKubeComponent, profilesKP.sharedKubeComponent)
	require.Same(t, traceKP.sharedWatchClient(), metricsKP.sharedWatchClient())
	require.Same(t, traceKP.sharedWatchClient(), logsKP.sharedWatchClient())
	require.Same(t, traceKP.sharedWatchClient(), profilesKP.sharedWatchClient())
	assert.Equal(t, 1, sharedWatchClientCount(t, factory))

	clients := clientProvider.snapshot()
	require.Len(t, clients, 1)
	starts, stops := clients[0].counts()
	assert.Equal(t, 1, starts)
	assert.Equal(t, 0, stops)
	assert.Equal(t, traceKP.sharedWatchClient().instanceID, metricsKP.sharedWatchClient().instanceID)
	assert.Equal(t, traceKP.sharedWatchClient().instanceID, logsKP.sharedWatchClient().instanceID)
	assert.Equal(t, traceKP.sharedWatchClient().instanceID, profilesKP.sharedWatchClient().instanceID)

	shutdownAllProcessors(t, tp, mp, lp, pp)

	starts, stops = clients[0].counts()
	assert.Equal(t, 1, starts)
	assert.Equal(t, 1, stops)
	assert.Equal(t, 0, sharedWatchClientCount(t, factory))
}

func TestSharedWatchClientFeatureGateDisabledKeepsDedicatedClients(t *testing.T) {
	setShareInformerCachesFeatureGate(t, false)

	factory := newTestFactory()
	clientProvider := &countingClientProvider{}
	cfg := NewFactory().CreateDefaultConfig()

	tp, mp, lp, pp, traceKP, metricsKP, logsKP, profilesKP := createAllSignalProcessors(
		t,
		factory,
		cfg,
		withKubeClientProvider(clientProvider.provider),
	)
	startAllProcessors(t, tp, mp, lp, pp)
	t.Cleanup(func() {
		shutdownAllProcessors(t, tp, mp, lp, pp)
	})

	assert.Nil(t, traceKP.sharedKubeComponent)
	assert.Nil(t, metricsKP.sharedKubeComponent)
	assert.Nil(t, logsKP.sharedKubeComponent)
	assert.Nil(t, profilesKP.sharedKubeComponent)
	assert.NotSame(t, traceKP.kc, metricsKP.kc)
	assert.NotSame(t, traceKP.kc, logsKP.kc)
	assert.NotSame(t, traceKP.kc, profilesKP.kc)
	assert.Equal(t, 0, sharedWatchClientCount(t, factory))

	clients := clientProvider.snapshot()
	require.Len(t, clients, 4)
	for _, client := range clients {
		starts, stops := client.counts()
		assert.Equal(t, 1, starts)
		assert.Equal(t, 0, stops)
	}
}

func TestSharedWatchClientDifferentConfigsDoNotShare(t *testing.T) {
	setShareInformerCachesFeatureGate(t, true)

	factory := newTestFactory()
	clientProvider := &countingClientProvider{}
	cfgA := NewFactory().CreateDefaultConfig()
	cfgB := createDefaultConfig().(*Config)
	cfgB.WaitForMetadata = true

	var traceKP, metricsKP *kubernetesprocessor
	tp, err := newTracesProcessorWithFactory(factory, cfgA, consumertest.NewNop(), withKubeClientProvider(clientProvider.provider), withExtractKubernetesProcessorInto(&traceKP))
	require.NoError(t, err)
	mp, err := newMetricsProcessorWithFactory(factory, cfgB, consumertest.NewNop(), withKubeClientProvider(clientProvider.provider), withExtractKubernetesProcessorInto(&metricsKP))
	require.NoError(t, err)
	startAllProcessors(t, tp, mp)
	t.Cleanup(func() {
		shutdownAllProcessors(t, tp, mp)
	})

	require.NotNil(t, traceKP.sharedKubeComponent)
	require.NotNil(t, metricsKP.sharedKubeComponent)
	assert.NotSame(t, traceKP.sharedKubeComponent, metricsKP.sharedKubeComponent)
	assert.NotEqual(t, traceKP.sharedWatchClient().instanceID, metricsKP.sharedWatchClient().instanceID)
	assert.Equal(t, 2, sharedWatchClientCount(t, factory))
	assert.Len(t, clientProvider.snapshot(), 2)
}

func TestSharedWatchClientOrderSensitiveKeyDoesNotShare(t *testing.T) {
	setShareInformerCachesFeatureGate(t, true)

	factory := newTestFactory()
	clientProvider := &countingClientProvider{}
	cfgA := createDefaultConfig().(*Config)
	cfgA.Extract.Metadata = []string{"k8s.pod.name", "k8s.namespace.name"}
	cfgB := createDefaultConfig().(*Config)
	cfgB.Extract.Metadata = []string{"k8s.namespace.name", "k8s.pod.name"}

	var traceKP, metricsKP *kubernetesprocessor
	tp, err := newTracesProcessorWithFactory(factory, cfgA, consumertest.NewNop(), withKubeClientProvider(clientProvider.provider), withExtractKubernetesProcessorInto(&traceKP))
	require.NoError(t, err)
	mp, err := newMetricsProcessorWithFactory(factory, cfgB, consumertest.NewNop(), withKubeClientProvider(clientProvider.provider), withExtractKubernetesProcessorInto(&metricsKP))
	require.NoError(t, err)
	startAllProcessors(t, tp, mp)
	t.Cleanup(func() {
		shutdownAllProcessors(t, tp, mp)
	})

	assert.NotSame(t, traceKP.sharedKubeComponent, metricsKP.sharedKubeComponent)
	assert.Equal(t, 2, sharedWatchClientCount(t, factory))
	assert.Len(t, clientProvider.snapshot(), 2)
}

func TestSharedWatchClientPassthroughBypassesSharedComponent(t *testing.T) {
	setShareInformerCachesFeatureGate(t, true)

	factory := newTestFactory()
	clientProvider := &countingClientProvider{}
	cfg := createDefaultConfig().(*Config)
	cfg.Passthrough = true

	var traceKP *kubernetesprocessor
	tp, err := newTracesProcessorWithFactory(factory, cfg, consumertest.NewNop(), withKubeClientProvider(clientProvider.provider), withExtractKubernetesProcessorInto(&traceKP))
	require.NoError(t, err)
	require.NoError(t, tp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(t.Context()))
	})

	assert.Nil(t, traceKP.sharedKubeComponent)
	assert.Nil(t, traceKP.sharedWatchClient())
	assert.Nil(t, traceKP.kc)
	assert.Equal(t, 0, sharedWatchClientCount(t, factory))
	assert.Empty(t, clientProvider.snapshot())
}
