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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type LogsDataConsumer interface {
	ConsumeLogsJSON(ctx context.Context, json []byte) error
	SetNextLogsConsumer(nextLogsConsumer consumer.Logs)
}

type TracesDataConsumer interface {
	ConsumeTracesJSON(ctx context.Context, json []byte) error
	SetNextTracesConsumer(nextracesConsumer consumer.Traces)
}

type blobReceiver struct {
	blobEventHandler   BlobEventHandler
	logger             *zap.Logger
	logsUnmarshaler    plog.Unmarshaler
	tracesUnmarshaler  ptrace.Unmarshaler
	nextLogsConsumer   consumer.Logs
	nextTracesConsumer consumer.Traces
	obsrecv            *obsreport.Receiver
}

func (b *blobReceiver) Start(ctx context.Context, host component.Host) error {

	b.blobEventHandler.SetLogsDataConsumer(b)
	b.blobEventHandler.SetTracesDataConsumer(b)

	err := b.blobEventHandler.Run(ctx)

	return err
}

func (b *blobReceiver) Shutdown(ctx context.Context) error {
	b.blobEventHandler.Close(ctx)

	return nil
}
func (b *blobReceiver) SetNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	b.nextLogsConsumer = nextLogsConsumer
}

func (b *blobReceiver) SetNextTracesConsumer(nextTracesConsumer consumer.Traces) {
	b.nextTracesConsumer = nextTracesConsumer
}

func (b *blobReceiver) ConsumeLogsJSON(ctx context.Context, json []byte) error {

	if b.nextLogsConsumer == nil {
		return nil
	}

	logsContext := b.obsrecv.StartLogsOp(ctx)

	logs, err := b.logsUnmarshaler.UnmarshalLogs(json)
	if err == nil {
		err = b.nextLogsConsumer.ConsumeLogs(logsContext, logs)
	} else {
		b.logger.Error(err.Error())
	}

	b.obsrecv.EndLogsOp(logsContext, typeStr, 1, err)

	return err
}

func (b *blobReceiver) ConsumeTracesJSON(ctx context.Context, json []byte) error {
	if b.nextTracesConsumer == nil {
		return nil
	}

	tracesContext := b.obsrecv.StartTracesOp(ctx)

	traces, err := b.tracesUnmarshaler.UnmarshalTraces(json)
	if err == nil {
		err = b.nextTracesConsumer.ConsumeTraces(tracesContext, traces)
	} else {
		b.logger.Error(err.Error())
	}

	b.obsrecv.EndTracesOp(tracesContext, typeStr, 1, err)

	return err
}

// Returns a new instance of the log receiver
func NewReceiver(config Config, set component.ReceiverCreateSettings, blobEventHandler BlobEventHandler) (component.Receiver, error) {
	blobReceiver := &blobReceiver{
		blobEventHandler:  blobEventHandler,
		logger:            set.Logger,
		logsUnmarshaler:   plog.NewJSONUnmarshaler(),
		tracesUnmarshaler: ptrace.NewJSONUnmarshaler(),
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              "event",
			ReceiverCreateSettings: set,
		}),
	}

	return blobReceiver, nil
}
