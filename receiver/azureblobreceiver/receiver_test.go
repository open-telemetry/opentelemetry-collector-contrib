// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

var (
	logsJSON   = []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"dotnet"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"timeUnixNano":"1643240673066096200","severityText":"Information","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"}]}]}]}`)
	tracesJSON = []byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"FilterModule"}},{"key":"service.instance.id","value":{"stringValue":"7020adec-62a0-4cc6-9878-876fec16e961"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.language","value":{"stringValue":"dotnet"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.2.0.268"}}]},"scopeSpans":[{"scope":{"name":"IoTSample.FilterModule"},"spans":[{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"4951aca774f96da6","parentSpanId":"c4ad1e000ef8bb19","name":"Upstream","kind":"SPAN_KIND_CLIENT","startTimeUnixNano":"1645750319674336500","endTimeUnixNano":"1645750319678074800","status":{}},{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"c4ad1e000ef8bb19","parentSpanId":"22ba720ac011163b","name":"FilterTemperature","kind":"SPAN_KIND_SERVER","startTimeUnixNano":"1645750319674157400","endTimeUnixNano":"1645750319678086500","attributes":[{"key":"MessageString","value":{"stringValue":"{\"machine\":{\"temperature\":100.2142046553614,\"pressure\":10.024403062003197},\"ambient\":{\"temperature\":20.759989948598662,\"humidity\":24},\"timeCreated\":\"2022-02-25T00:51:59.6685152Z\"}"}},{"key":"MachineTemperature","value":{"doubleValue":100.2142046553614}},{"key":"TemperatureThreshhold","value":{"intValue":"25"}}],"events":[{"timeUnixNano":"1645750319674315300","name":"Machine temperature 100.2142046553614 exceeds threshold 25"},{"timeUnixNano":"1645750319674324100","name":"Message passed threshold"}],"status":{}}]}]}]}`)
)

var testModes = []string{"eventhub", "polling"}

func TestNewReceiver(t *testing.T) {
	for _, mode := range testModes {
		t.Run(mode, func(tt *testing.T) {
			receiver, err := getBlobReceiver(tt, mode)

			require.NoError(tt, err)

			assert.NotNil(tt, receiver)
		})
	}
}

func TestConsumeLogsJSON(t *testing.T) {
	for _, mode := range testModes {
		t.Run(mode, func(tt *testing.T) {
			receiver, _ := getBlobReceiver(tt, mode)

			logsSink := new(consumertest.LogsSink)
			logsConsumer, ok := receiver.(logsDataConsumer)
			require.True(tt, ok)

			logsConsumer.setNextLogsConsumer(logsSink)

			err := logsConsumer.consumeLogs(tt.Context(), logsJSON)
			require.NoError(tt, err)
			assert.Equal(tt, 1, logsSink.LogRecordCount())
		})
	}
}

func TestConsumeTracesJSON(t *testing.T) {
	for _, mode := range testModes {
		t.Run(mode, func(tt *testing.T) {
			receiver, _ := getBlobReceiver(tt, mode)

			tracesSink := new(consumertest.TracesSink)
			tracesConsumer, ok := receiver.(tracesDataConsumer)
			require.True(tt, ok)

			tracesConsumer.setNextTracesConsumer(tracesSink)

			err := tracesConsumer.consumeTraces(tt.Context(), tracesJSON)
			require.NoError(tt, err)
			assert.Equal(tt, 2, tracesSink.SpanCount())
		})
	}
}

func TestConsumeLogsProto(t *testing.T) {
	jsonUnmarshaler := &plog.JSONUnmarshaler{}
	protoMarshaler := &plog.ProtoMarshaler{}

	logs, err := jsonUnmarshaler.UnmarshalLogs(logsJSON)
	require.NoError(t, err)
	logsProto, err := protoMarshaler.MarshalLogs(logs)
	require.NoError(t, err)

	for _, mode := range testModes {
		t.Run(mode, func(tt *testing.T) {
			receiver, _ := getBlobReceiverWithEncodings(tt, mode, EncodingOTLPProto, EncodingOTLPJSON)

			logsSink := new(consumertest.LogsSink)
			logsConsumer, ok := receiver.(logsDataConsumer)
			require.True(tt, ok)

			logsConsumer.setNextLogsConsumer(logsSink)

			err := logsConsumer.consumeLogs(tt.Context(), logsProto)
			require.NoError(tt, err)
			assert.Equal(tt, 1, logsSink.LogRecordCount())
		})
	}
}

func TestConsumeTracesProto(t *testing.T) {
	jsonUnmarshaler := &ptrace.JSONUnmarshaler{}
	protoMarshaler := &ptrace.ProtoMarshaler{}

	traces, err := jsonUnmarshaler.UnmarshalTraces(tracesJSON)
	require.NoError(t, err)
	tracesProto, err := protoMarshaler.MarshalTraces(traces)
	require.NoError(t, err)

	for _, mode := range testModes {
		t.Run(mode, func(tt *testing.T) {
			receiver, _ := getBlobReceiverWithEncodings(tt, mode, EncodingOTLPJSON, EncodingOTLPProto)

			tracesSink := new(consumertest.TracesSink)
			tracesConsumer, ok := receiver.(tracesDataConsumer)
			require.True(tt, ok)

			tracesConsumer.setNextTracesConsumer(tracesSink)

			err := tracesConsumer.consumeTraces(tt.Context(), tracesProto)
			require.NoError(tt, err)
			assert.Equal(tt, 2, tracesSink.SpanCount())
		})
	}
}

func TestConsumeLogsEncoding(t *testing.T) {
	jsonUnmarshaler := &plog.JSONUnmarshaler{}
	protoMarshaler := &plog.ProtoMarshaler{}

	logs, err := jsonUnmarshaler.UnmarshalLogs(logsJSON)
	require.NoError(t, err)
	logsProto, err := protoMarshaler.MarshalLogs(logs)
	require.NoError(t, err)

	tests := []struct {
		name      string
		encoding  string
		payload   []byte
		expectErr bool
	}{
		{name: "json encoding accepts json", encoding: EncodingOTLPJSON, payload: logsJSON, expectErr: false},
		{name: "json encoding rejects proto", encoding: EncodingOTLPJSON, payload: logsProto, expectErr: true},
		{name: "proto encoding accepts proto", encoding: EncodingOTLPProto, payload: logsProto, expectErr: false},
		{name: "proto encoding rejects json", encoding: EncodingOTLPProto, payload: logsJSON, expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receiver, err := getBlobReceiverWithEncodings(t, "polling", tc.encoding, EncodingOTLPJSON)
			require.NoError(t, err)

			logsSink := new(consumertest.LogsSink)
			logsConsumer := receiver.(logsDataConsumer)
			logsConsumer.setNextLogsConsumer(logsSink)

			err = logsConsumer.consumeLogs(t.Context(), tc.payload)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, logsSink.LogRecordCount())
			}
		})
	}
}

func TestNewReceiverUnsupportedEncoding(t *testing.T) {
	tests := []struct {
		name           string
		logsEncoding   string
		tracesEncoding string
	}{
		{name: "unsupported logs encoding", logsEncoding: "totally_made_up", tracesEncoding: EncodingOTLPJSON},
		{name: "unsupported traces encoding", logsEncoding: EncodingOTLPJSON, tracesEncoding: "totally_made_up"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := getBlobReceiverWithEncodings(t, "polling", tc.logsEncoding, tc.tracesEncoding)
			require.Error(t, err)
		})
	}
}

func getBlobReceiver(t *testing.T, mode string) (component.Component, error) {
	return getBlobReceiverWithEncodings(t, mode, EncodingOTLPJSON, EncodingOTLPJSON)
}

func getBlobReceiverWithEncodings(t *testing.T, mode, logsEncoding, tracesEncoding string) (component.Component, error) {
	set := receivertest.NewNopSettings(metadata.Type)
	if mode == "eventhub" {
		blobClient := newMockBlobClient()
		blobEventHandler := getEventHubEventHandler(t, blobClient)
		return newReceiver(set, blobEventHandler, logsEncoding, tracesEncoding)
	}
	if mode == "polling" {
		blobClient := newMockBlobClient()
		blobEventHandler := getBlobEventHandler(t, blobClient)
		return newReceiver(set, blobEventHandler, logsEncoding, tracesEncoding)
	}
	return nil, errors.New("invalid mode")
}
