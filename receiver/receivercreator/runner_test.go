// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
)

func Test_loadAndCreateMetricsRuntimeReceiver(t *testing.T) {
	logCore, logs := observer.New(zap.DebugLevel)
	logger := zap.New(logCore).With(zap.String("name", "receiver_creator"))
	rcs := receivertest.NewNopSettings(metadata.Type)
	rcs.Logger = logger
	run := &receiverRunner{params: rcs, idNamespace: component.NewIDWithName(metadata.Type, "1")}
	exampleFactory := &nopWithEndpointFactory{}
	template, err := newReceiverTemplate("nop/1", nil)
	require.NoError(t, err)

	loadedConfig, endpoint, err := run.loadRuntimeReceiverConfig(exampleFactory, template.receiverConfig, userConfigMap{
		tmpSetEndpointConfigKey: struct{}{},
		endpointConfigKey:       "localhost:12345",
	})
	require.NoError(t, err)
	assert.Equal(t, "localhost:12345", endpoint)
	require.NotNil(t, loadedConfig)
	nopConfig := loadedConfig.(*nopWithEndpointConfig)
	// Verify that the overridden endpoint is used instead of the one in the config file.
	assert.Equal(t, "localhost:12345", nopConfig.Endpoint)
	expectedID := `nop/1/receiver_creator/1{endpoint="localhost:12345"}/endpoint.id`

	// Test that metric receiver can be created from loaded config and it logs its id for the "name" field.
	t.Run("test create receiver from loaded config", func(t *testing.T) {
		recvr, err := run.createMetricsRuntimeReceiver(
			exampleFactory,
			component.MustNewIDWithName("nop", "1/receiver_creator/1{endpoint=\"localhost:12345\"}/endpoint.id"),
			loadedConfig,
			nil,
		)
		require.NoError(t, err)
		assert.NotNil(t, recvr)
		assert.IsType(t, &nopWithEndpointReceiver{}, recvr)
		recvr.(*nopWithEndpointReceiver).Logger.Warn("test message")
		assert.True(t, func() bool {
			var found bool
			for _, entry := range logs.All() {
				if name, ok := entry.ContextMap()["name"]; ok {
					found = true
					assert.Equal(t, expectedID, name)
				}
			}
			return found
		}())
	})
}

func TestValidateSetEndpointFromConfig(t *testing.T) {
	type configWithEndpoint struct {
		Endpoint any `mapstructure:"endpoint"`
	}

	receiverWithEndpoint := receiver.NewFactory(component.MustNewType("with_endpoint"), func() component.Config {
		return &configWithEndpoint{}
	})

	type configWithoutEndpoint struct {
		NotEndpoint any `mapstructure:"not.endpoint"`
	}

	receiverWithoutEndpoint := receiver.NewFactory(component.MustNewType("without_endpoint"), func() component.Config {
		return &configWithoutEndpoint{}
	})

	setEndpointConfMap, setEndpoint, setErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, nil, map[string]any{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "an.endpoint",
		},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr := mergeTemplatedAndDiscoveredConfigs(
		receiverWithEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)

	setEndpointConfMap, setEndpoint, setErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, nil, map[string]any{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "an.endpoint",
		},
	)
	require.Equal(t, map[string]any{}, setEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", setEndpoint)
	require.NoError(t, setErr)

	inheritedEndpointConfMap, inheritedEndpoint, inheritedErr = mergeTemplatedAndDiscoveredConfigs(
		receiverWithoutEndpoint, map[string]any{
			endpointConfigKey: "an.endpoint",
		}, map[string]any{},
	)
	require.Equal(t, map[string]any{endpointConfigKey: "an.endpoint"}, inheritedEndpointConfMap.ToStringMap())
	require.Equal(t, "an.endpoint", inheritedEndpoint)
	require.NoError(t, inheritedErr)
}

// erroringReceiver always fails to start, to exercise the
// StatusRecoverableError path of reportSubReceiverStatus.
type erroringReceiver struct{}

func (*erroringReceiver) Start(context.Context, component.Host) error {
	return errors.New("intentional start failure")
}

func (*erroringReceiver) Shutdown(context.Context) error {
	return nil
}

type erroringFactory struct {
	receiver.Factory
}

func (*erroringFactory) CreateDefaultConfig() component.Config {
	return &nopWithEndpointConfig{}
}

func (*erroringFactory) CreateMetrics(context.Context, receiver.Settings, component.Config, consumer.Metrics) (receiver.Metrics, error) {
	return &erroringReceiver{}, nil
}

// reportingMockHost gives componentstatus.ReportStatus a Report method to
// find: mockHost embeds component.Host as an interface field, so it only
// promotes that interface's own methods, not Report from whatever concrete
// value happens to be stored in it.
type reportingMockHost struct {
	*mockHost
	reporter *reportingHost
}

func (h *reportingMockHost) Report(ev *componentstatus.Event) {
	h.reporter.Report(ev)
}

func newReportingMockHost(t *testing.T, onReport func(ev *componentstatus.Event)) *reportingMockHost {
	reporter := &reportingHost{reportFunc: onReport}
	return &reportingMockHost{mockHost: newMockHost(t, reporter), reporter: reporter}
}

func TestReceiverRunner_ReportsSubReceiverStatus(t *testing.T) {
	t.Run("reports StatusOK with the subcomponent id on successful start", func(t *testing.T) {
		var events []*componentstatus.Event
		host := newReportingMockHost(t, func(ev *componentstatus.Event) { events = append(events, ev) })
		run := &receiverRunner{
			params:      receivertest.NewNopSettings(metadata.Type),
			idNamespace: component.NewIDWithName(metadata.Type, "1"),
			host:        host,
		}
		template, err := newReceiverTemplate("with_endpoint/1", nil)
		require.NoError(t, err)

		comp, err := run.start(template.receiverConfig, userConfigMap{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "localhost:12345",
		}, &enhancingConsumer{metrics: consumertest.NewNop()})
		require.NoError(t, err)
		require.NotNil(t, comp)

		require.Len(t, events, 1)
		assert.Equal(t, componentstatus.StatusOK, events[0].Status())
		subID, ok := events[0].Attributes().Get(subComponentIDAttr)
		require.True(t, ok)
		assert.Contains(t, subID.AsString(), run.idNamespace.String())
		assert.Contains(t, subID.AsString(), "endpoint.id")
	})

	t.Run("reports StatusRecoverableError with the subcomponent id when start fails", func(t *testing.T) {
		var events []*componentstatus.Event
		host := newReportingMockHost(t, func(ev *componentstatus.Event) { events = append(events, ev) })
		host.factories.Receivers[component.MustNewType("erroring")] = &erroringFactory{Factory: receivertest.NewNopFactory()}
		run := &receiverRunner{
			params:      receivertest.NewNopSettings(metadata.Type),
			idNamespace: component.NewIDWithName(metadata.Type, "1"),
			host:        host,
		}
		template, err := newReceiverTemplate("erroring/1", nil)
		require.NoError(t, err)

		comp, err := run.start(template.receiverConfig, userConfigMap{
			tmpSetEndpointConfigKey: struct{}{},
			endpointConfigKey:       "localhost:12345",
		}, &enhancingConsumer{metrics: consumertest.NewNop()})
		require.Error(t, err)
		require.Nil(t, comp)

		require.Len(t, events, 1)
		assert.Equal(t, componentstatus.StatusRecoverableError, events[0].Status())
		require.Error(t, events[0].Err())
		subID, ok := events[0].Attributes().Get(subComponentIDAttr)
		require.True(t, ok)
		assert.Contains(t, subID.AsString(), run.idNamespace.String())
		assert.Contains(t, subID.AsString(), "endpoint.id")
	})
}
