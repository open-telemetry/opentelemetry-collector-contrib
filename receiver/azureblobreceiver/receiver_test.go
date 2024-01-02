// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

var (
	logsJSON   = []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"dotnet"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"timeUnixNano":"1643240673066096200","severityText":"Information","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"}]}]}]}`)
	tracesJSON = []byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"FilterModule"}},{"key":"service.instance.id","value":{"stringValue":"7020adec-62a0-4cc6-9878-876fec16e961"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.language","value":{"stringValue":"dotnet"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.2.0.268"}}]},"scopeSpans":[{"scope":{"name":"IoTSample.FilterModule"},"spans":[{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"4951aca774f96da6","parentSpanId":"c4ad1e000ef8bb19","name":"Upstream","kind":"SPAN_KIND_CLIENT","startTimeUnixNano":"1645750319674336500","endTimeUnixNano":"1645750319678074800","status":{}},{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"c4ad1e000ef8bb19","parentSpanId":"22ba720ac011163b","name":"FilterTemperature","kind":"SPAN_KIND_SERVER","startTimeUnixNano":"1645750319674157400","endTimeUnixNano":"1645750319678086500","attributes":[{"key":"MessageString","value":{"stringValue":"{\"machine\":{\"temperature\":100.2142046553614,\"pressure\":10.024403062003197},\"ambient\":{\"temperature\":20.759989948598662,\"humidity\":24},\"timeCreated\":\"2022-02-25T00:51:59.6685152Z\"}"}},{"key":"MachineTemperature","value":{"doubleValue":100.2142046553614}},{"key":"TemperatureThreshhold","value":{"intValue":"25"}}],"events":[{"timeUnixNano":"1645750319674315300","name":"Machine temperature 100.2142046553614 exceeds threshold 25"},{"timeUnixNano":"1645750319674324100","name":"Message passed threshold"}],"status":{}}]}]}]}`)
)

func TestNewReceiver(t *testing.T) {
	receiver, err := getBlobReceiver(t)

	require.NoError(t, err)

	assert.NotNil(t, receiver)
}

func TestConsumeLogsJSON(t *testing.T) {
	receiver, _ := getBlobReceiver(t)

	logsSink := new(consumertest.LogsSink)
	logsConsumer, ok := receiver.(logsDataConsumer)
	require.True(t, ok)

	logsConsumer.setNextLogsConsumer(logsSink)

	err := logsConsumer.consumeLogsJSON(context.Background(), logsJSON)
	require.NoError(t, err)
	assert.Equal(t, logsSink.LogRecordCount(), 1)
}

func TestConsumeTracesJSON(t *testing.T) {
	receiver, _ := getBlobReceiver(t)

	tracesSink := new(consumertest.TracesSink)
	tracesConsumer, ok := receiver.(tracesDataConsumer)
	require.True(t, ok)

	tracesConsumer.setNextTracesConsumer(tracesSink)

	err := tracesConsumer.consumeTracesJSON(context.Background(), tracesJSON)
	require.NoError(t, err)
	assert.Equal(t, tracesSink.SpanCount(), 2)
}

func getBlobReceiver(t *testing.T) (component.Component, error) {
	set := receivertest.NewNopCreateSettings()

	blobClient := newMockBlobClient()
	blobEventHandler := getBlobEventHandler(t, blobClient)

	getBlobEventHandler(t, blobClient)
	return newReceiver(set, blobEventHandler)
}
