// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

type mockObserver struct{}

func (m *mockObserver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (m *mockObserver) Shutdown(_ context.Context) error {
	return nil
}

var _ extension.Extension = (*mockObserver)(nil)

func (m *mockObserver) ListAndWatch(notify observer.Notify) {
	notify.OnAdd([]observer.Endpoint{portEndpoint})
}

func (m *mockObserver) Unsubscribe(_ observer.Notify) {}

var _ observer.Observable = (*mockObserver)(nil)

func TestMockedEndToEnd(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factories, _ := otelcoltest.NopFactories()
	factories.Receivers[component.MustNewType("nop")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}
	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory

	host := &mockHostFactories{Host: componenttest.NewNopHost(), factories: factories}
	host.extensions = map[component.ID]component.Component{
		component.MustNewID("mock_observer"):                      &mockObserver{},
		component.MustNewIDWithName("mock_observer", "with_name"): &mockObserver{},
	}

	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "1").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := receivertest.NewNopSettings(metadata.Type)
	mockConsumer := new(consumertest.MetricsSink)

	rcvr, err := factory.CreateMetrics(context.Background(), params, cfg, mockConsumer)
	require.NoError(t, err)
	sc := rcvr.(*sharedcomponent.SharedComponent)
	dyn := sc.Component.(*receiverCreator)
	require.NoError(t, rcvr.Start(context.Background(), host))

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			assert.NoError(t, rcvr.Shutdown(context.Background()))
		})
	}

	defer shutdown()

	require.Eventuallyf(t, func() bool {
		return dyn.observerHandler.receiversByEndpointID.Size() == 2
	}, 1*time.Second, 100*time.Millisecond, "expected 2 receiver but got %v", dyn.observerHandler.receiversByEndpointID)

	// Test that we can send metrics.
	for _, receiver := range dyn.observerHandler.receiversByEndpointID.Values() {
		wr := receiver.(*wrappedReceiver)
		example := wr.metrics.(*nopWithEndpointReceiver)
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("attr", "1")
		rm.Resource().Attributes().PutStr(string(semconv.ServiceNameKey), "dynamictest")
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("my-metric")
		m.SetDescription("My metric")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(123)
		assert.NoError(t, example.ConsumeMetrics(context.Background(), md))
	}

	// TODO: Will have to rework once receivers are started asynchronously to Start().
	assert.Len(t, mockConsumer.AllMetrics(), 2)
}
