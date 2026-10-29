// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver/internal/metadata"
)

const testConnection = "Endpoint=sb://namespace.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=superSecret1234=;EntityPath=hubName"

type mockEncodingExtension struct{}

func (mockEncodingExtension) Start(context.Context, component.Host) error { return nil }
func (mockEncodingExtension) Shutdown(context.Context) error              { return nil }

func (mockEncodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(string(buf))
	return logs, nil
}

func (mockEncodingExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(string(buf))
	return metrics, nil
}

func (mockEncodingExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName(string(buf))
	return traces, nil
}

type mockBareExtension struct{}

func (mockBareExtension) Start(context.Context, component.Host) error { return nil }
func (mockBareExtension) Shutdown(context.Context) error              { return nil }

type mockEncodingHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h mockEncodingHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func newTestEvent(data string) *azureEvent {
	return &azureEvent{
		AzEventData: &azeventhubs.ReceivedEventData{
			EventData: azeventhubs.EventData{
				Body: []byte(data),
			},
		},
	}
}

func TestEncodingLogsUnmarshaler(t *testing.T) {
	u := encodingLogsUnmarshaler{unmarshaler: mockEncodingExtension{}}
	logs, err := u.UnmarshalLogs(newTestEvent("hello"))
	require.NoError(t, err)
	require.Equal(t, 1, logs.LogRecordCount())
	assert.Equal(t, "hello", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}

func TestEncodingMetricsUnmarshaler(t *testing.T) {
	u := encodingMetricsUnmarshaler{unmarshaler: mockEncodingExtension{}}
	metrics, err := u.UnmarshalMetrics(newTestEvent("hello"))
	require.NoError(t, err)
	require.Equal(t, 1, metrics.MetricCount())
	assert.Equal(t, "hello", metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
}

func TestEncodingTracesUnmarshaler(t *testing.T) {
	u := encodingTracesUnmarshaler{unmarshaler: mockEncodingExtension{}}
	traces, err := u.UnmarshalTraces(newTestEvent("hello"))
	require.NoError(t, err)
	require.Equal(t, 1, traces.SpanCount())
	assert.Equal(t, "hello", traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
}

func TestSetEncodingUnmarshaler(t *testing.T) {
	encodingID := component.MustNewID("myencoding")

	host := mockEncodingHost{
		extensions: map[component.ID]component.Component{
			encodingID: mockEncodingExtension{},
		},
	}

	t.Run("logs", func(t *testing.T) {
		r := &eventhubReceiver{signal: pipeline.SignalLogs, encodingID: &encodingID}
		require.NoError(t, r.setEncodingUnmarshaler(host))
		assert.IsType(t, encodingLogsUnmarshaler{}, r.logsUnmarshaler)
	})

	t.Run("metrics", func(t *testing.T) {
		r := &eventhubReceiver{signal: pipeline.SignalMetrics, encodingID: &encodingID}
		require.NoError(t, r.setEncodingUnmarshaler(host))
		assert.IsType(t, encodingMetricsUnmarshaler{}, r.metricsUnmarshaler)
	})

	t.Run("traces", func(t *testing.T) {
		r := &eventhubReceiver{signal: pipeline.SignalTraces, encodingID: &encodingID}
		require.NoError(t, r.setEncodingUnmarshaler(host))
		assert.IsType(t, encodingTracesUnmarshaler{}, r.tracesUnmarshaler)
	})
}

func TestSetEncodingUnmarshaler_ExtensionNotFound(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{extensions: map[component.ID]component.Component{}}

	r := &eventhubReceiver{signal: pipeline.SignalLogs, encodingID: &encodingID}
	err := r.setEncodingUnmarshaler(host)
	assert.ErrorContains(t, err, "not found")
}

func TestSetEncodingUnmarshaler_WrongType(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{
		extensions: map[component.ID]component.Component{
			encodingID: mockBareExtension{},
		},
	}

	tests := []struct {
		signal      pipeline.Signal
		expectedErr string
	}{
		{pipeline.SignalLogs, "is not a logs unmarshaler"},
		{pipeline.SignalMetrics, "is not a metrics unmarshaler"},
		{pipeline.SignalTraces, "is not a traces unmarshaler"},
	}

	for _, tt := range tests {
		t.Run(tt.signal.String(), func(t *testing.T) {
			r := &eventhubReceiver{signal: tt.signal, encodingID: &encodingID}
			assert.ErrorContains(t, r.setEncodingUnmarshaler(host), tt.expectedErr)
		})
	}
}

func TestSetEncodingUnmarshaler_InvalidSignal(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{
		extensions: map[component.ID]component.Component{
			encodingID: mockEncodingExtension{},
		},
	}

	r := &eventhubReceiver{signal: pipeline.Signal{}, encodingID: &encodingID}
	assert.ErrorContains(t, r.setEncodingUnmarshaler(host), "invalid data type")
}

func TestStartResolvesEncodingExtension(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{
		extensions: map[component.ID]component.Component{
			encodingID: mockEncodingExtension{},
		},
	}

	config := createDefaultConfig().(*Config)
	config.Connection = testConnection
	config.Encoding = &encodingID

	settings := receivertest.NewNopSettings(metadata.Type)
	eventHandler := newEventhubHandler(config, settings)
	eventHandler.hub = &mockHubWrapper{}

	rcvr, err := newReceiver(pipeline.SignalLogs, &encodingID, nil, nil, nil, eventHandler, settings)
	require.NoError(t, err)

	r := rcvr.(*eventhubReceiver)
	require.Nil(t, r.logsUnmarshaler)

	require.NoError(t, r.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, r.Shutdown(t.Context())) })

	assert.IsType(t, encodingLogsUnmarshaler{}, r.logsUnmarshaler)
}

func TestStartWithMissingEncodingExtensionFails(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{extensions: map[component.ID]component.Component{}}

	config := createDefaultConfig().(*Config)
	config.Connection = testConnection
	config.Encoding = &encodingID

	settings := receivertest.NewNopSettings(metadata.Type)
	eventHandler := newEventhubHandler(config, settings)
	eventHandler.hub = &mockHubWrapper{}

	rcvr, err := newReceiver(pipeline.SignalLogs, &encodingID, nil, nil, nil, eventHandler, settings)
	require.NoError(t, err)

	assert.Error(t, rcvr.(*eventhubReceiver).Start(t.Context(), host))
}

func TestConsumeWithEncodingDecodesBody(t *testing.T) {
	encodingID := component.MustNewID("myencoding")
	host := mockEncodingHost{
		extensions: map[component.ID]component.Component{
			encodingID: mockEncodingExtension{},
		},
	}

	config := createDefaultConfig().(*Config)
	config.Connection = testConnection
	config.Encoding = &encodingID

	settings := receivertest.NewNopSettings(metadata.Type)
	eventHandler := newEventhubHandler(config, settings)
	eventHandler.hub = &mockHubWrapper{}

	sink := new(consumertest.LogsSink)
	rcvr, err := newReceiver(pipeline.SignalLogs, &encodingID, nil, nil, nil, eventHandler, settings)
	require.NoError(t, err)
	r := rcvr.(*eventhubReceiver)
	r.setNextLogsConsumer(sink)

	require.NoError(t, r.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, r.Shutdown(t.Context())) })

	require.NoError(t, r.consume(t.Context(), newTestEvent("payload")))

	require.Len(t, sink.AllLogs(), 1)
	got := sink.AllLogs()[0]
	require.Equal(t, 1, got.LogRecordCount())
	assert.Equal(t, "payload", got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}
