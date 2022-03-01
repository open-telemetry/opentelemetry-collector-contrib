// Copyright OpenTelemetry Authors
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

package azureblobexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap/zaptest"
)

var (
	testTraces = []byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"FilterModule"}},{"key":"service.instance.id","value":{"stringValue":"7020adec-62a0-4cc6-9878-876fec16e961"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.language","value":{"stringValue":"dotnet"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.2.0.268"}}]},"instrumentationLibrarySpans":[{"instrumentationLibrary":{"name":"IoTSample.FilterModule"},"spans":[{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"4951aca774f96da6","parentSpanId":"c4ad1e000ef8bb19","name":"Upstream","kind":"SPAN_KIND_CLIENT","startTimeUnixNano":"1645750319674336500","endTimeUnixNano":"1645750319678074800","status":{}},{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"c4ad1e000ef8bb19","parentSpanId":"22ba720ac011163b","name":"FilterTemperature","kind":"SPAN_KIND_SERVER","startTimeUnixNano":"1645750319674157400","endTimeUnixNano":"1645750319678086500","attributes":[{"key":"MessageString","value":{"stringValue":"{\"machine\":{\"temperature\":100.2142046553614,\"pressure\":10.024403062003197},\"ambient\":{\"temperature\":20.759989948598662,\"humidity\":24},\"timeCreated\":\"2022-02-25T00:51:59.6685152Z\"}"}},{"key":"MachineTemperature","value":{"doubleValue":100.2142046553614}},{"key":"TemperatureThreshhold","value":{"intValue":"25"}}],"events":[{"timeUnixNano":"1645750319674315300","name":"Machine temperature 100.2142046553614 exceeds threshold 25"},{"timeUnixNano":"1645750319674324100","name":"Message passed threshold"}],"status":{}}]}]}]}`)
)

// Test onLogData callback for the test logs data
func TestExporterTracesDataCallback(t *testing.T) {
	traces := getTestTraces(t)

	blobClient := NewMockBlobClient()

	exporter := getTracesExporter(t, blobClient)

	assert.NoError(t, exporter.onTraceData(context.Background(), traces))

	blobClient.AssertNumberOfCalls(t, "UploadData", 1)
}

func getTracesExporter(tb testing.TB, blobClient BlobClient) *traceExporter {
	exporter := &traceExporter{
		blobClient:      blobClient,
		logger:          zaptest.NewLogger(tb),
		tracesMarshaler: otlp.NewJSONTracesMarshaler(),
	}

	return exporter
}

func getTestTraces(tb testing.TB) pdata.Traces {
	tracesMarshaler := otlp.NewJSONTracesUnmarshaler()
	traces, err := tracesMarshaler.UnmarshalTraces(testTraces)
	require.NoError(tb, err, "Can't unmarshal testing traces data -> %s", err)
	return traces
}
