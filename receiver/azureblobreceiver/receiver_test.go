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

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

// type LogsDataConsumer interface {
// 	ConsumeLogsJSON(ctx context.Context, json []byte) error
// 	SetNextLogsConsumer(nextLogsConsumer consumer.Logs)
// }

// type TracesDataConsumer interface {
// 	ConsumeTracesJSON(ctx context.Context, json []byte) error
// 	SetNextTracesConsumer(nextracesConsumer consumer.Traces)
// }

// type blobReceiver struct {
// 	blobEventHandler   BlobEventHandler
// 	logger             *zap.Logger
// 	logsUnmarshaler    pdata.LogsUnmarshaler
// 	tracesUnmarshaler  pdata.TracesUnmarshaler
// 	nextLogsConsumer   consumer.Logs
// 	nextTracesConsumer consumer.Traces
// 	obsrecv            *obsreport.Receiver
// }

// func (b *blobReceiver) Start(ctx context.Context, host component.Host) error {

// 	b.blobEventHandler.SetLogsDataConsumer(b)
// 	b.blobEventHandler.SetTracesDataConsumer(b)

// 	b.blobEventHandler.Run(ctx)

// 	return nil
// }

// func (b *blobReceiver) Shutdown(ctx context.Context) error {
// 	b.blobEventHandler.Close(ctx)

// 	return nil
// }
// func (b *blobReceiver) SetNextLogsConsumer(nextLogsConsumer consumer.Logs) {
// 	b.nextLogsConsumer = nextLogsConsumer
// }

// func (b *blobReceiver) SetNextTracesConsumer(nextTracesConsumer consumer.Traces) {
// 	b.nextTracesConsumer = nextTracesConsumer
// }

// func (b *blobReceiver) ConsumeLogsJSON(ctx context.Context, json []byte) error {

// 	if b.nextLogsConsumer == nil {
// 		return nil
// 	}

// 	logsContext := b.obsrecv.StartLogsOp(ctx)

// 	logs, err := b.logsUnmarshaler.UnmarshalLogs(json)
// 	if err == nil {
// 		err = b.nextLogsConsumer.ConsumeLogs(logsContext, logs)
// 	} else {
// 		b.logger.Error(err.Error())
// 	}

// 	b.obsrecv.EndLogsOp(logsContext, typeStr, 1, err)

// 	return err
// }

// func (b *blobReceiver) ConsumeTracesJSON(ctx context.Context, json []byte) error {
// 	if b.nextTracesConsumer == nil {
// 		return nil
// 	}

// 	tracesContext := b.obsrecv.StartTracesOp(ctx)

// 	traces, err := b.tracesUnmarshaler.UnmarshalTraces(json)
// 	if err == nil {
// 		err = b.nextTracesConsumer.ConsumeTraces(tracesContext, traces)
// 	} else {
// 		b.logger.Error(err.Error())
// 	}

// 	b.obsrecv.EndTracesOp(tracesContext, typeStr, 1, err)

// 	return err
// }

// // Returns a new instance of the log receiver
// func NewReceiver(config Config, set component.ReceiverCreateSettings, blobEventHandler BlobEventHandler) (component.LogsReceiver, error) {
// 	blobReceiver := &blobReceiver{
// 		blobEventHandler:  blobEventHandler,
// 		logger:            set.Logger,
// 		logsUnmarshaler:   otlp.NewJSONLogsUnmarshaler(),
// 		tracesUnmarshaler: otlp.NewJSONTracesUnmarshaler(),
// 		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
// 			ReceiverID:             config.ID(),
// 			Transport:              "event",
// 			ReceiverCreateSettings: set,
// 		}),
// 	}

// 	return blobReceiver, nil
// }

var (
	logsJSON   = []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"dotnet"}}]},"instrumentationLibraryLogs":[{"instrumentationLibrary":{},"logRecords":[{"timeUnixNano":"1643240673066096200","severityText":"Information","name":"FilterModule.Program","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"}]}]}]}`)
	tracesJSON = []byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"FilterModule"}},{"key":"service.instance.id","value":{"stringValue":"7020adec-62a0-4cc6-9878-876fec16e961"}},{"key":"telemetry.sdk.name","value":{"stringValue":"opentelemetry"}},{"key":"telemetry.sdk.language","value":{"stringValue":"dotnet"}},{"key":"telemetry.sdk.version","value":{"stringValue":"1.2.0.268"}}]},"instrumentationLibrarySpans":[{"instrumentationLibrary":{"name":"IoTSample.FilterModule"},"spans":[{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"4951aca774f96da6","parentSpanId":"c4ad1e000ef8bb19","name":"Upstream","kind":"SPAN_KIND_CLIENT","startTimeUnixNano":"1645750319674336500","endTimeUnixNano":"1645750319678074800","status":{}},{"traceId":"b8c21a8a5aeb50b98f38d548cedc7068","spanId":"c4ad1e000ef8bb19","parentSpanId":"22ba720ac011163b","name":"FilterTemperature","kind":"SPAN_KIND_SERVER","startTimeUnixNano":"1645750319674157400","endTimeUnixNano":"1645750319678086500","attributes":[{"key":"MessageString","value":{"stringValue":"{\"machine\":{\"temperature\":100.2142046553614,\"pressure\":10.024403062003197},\"ambient\":{\"temperature\":20.759989948598662,\"humidity\":24},\"timeCreated\":\"2022-02-25T00:51:59.6685152Z\"}"}},{"key":"MachineTemperature","value":{"doubleValue":100.2142046553614}},{"key":"TemperatureThreshhold","value":{"intValue":"25"}}],"events":[{"timeUnixNano":"1645750319674315300","name":"Machine temperature 100.2142046553614 exceeds threshold 25"},{"timeUnixNano":"1645750319674324100","name":"Message passed threshold"}],"status":{}}]}]}]}`)
)

func TestNewReceiver(t *testing.T) {
	receiver, err := getBlobReceiver()

	require.NoError(t, err)

	assert.NotNil(t, receiver)
}

func TestConsumeLogsJSON(t *testing.T) {
	receiver, _ := getBlobReceiver()

	logsSink := new(consumertest.LogsSink)
	logsConsumer, ok := receiver.(LogsDataConsumer)
	require.True(t, ok)

	logsConsumer.SetNextLogsConsumer(logsSink)

	err := logsConsumer.ConsumeLogsJSON(context.Background(), logsJSON)
	require.NoError(t, err)
	assert.Equal(t, logsSink.LogRecordCount(), 1)
}

func TestConsumeTracesJSON(t *testing.T) {
	receiver, _ := getBlobReceiver()

	tracesSink := new(consumertest.TracesSink)
	tracesConsumer, ok := receiver.(TracesDataConsumer)
	require.True(t, ok)

	tracesConsumer.SetNextTracesConsumer(tracesSink)

	err := tracesConsumer.ConsumeTracesJSON(context.Background(), tracesJSON)
	require.NoError(t, err)
	assert.Equal(t, tracesSink.SpanCount(), 2)
}

func getBlobReceiver() (component.Receiver, error) {
	set := componenttest.NewNopReceiverCreateSettings()
	cfg := getConfig().(*Config)
	return NewReceiver(*cfg, set, nil)
}
