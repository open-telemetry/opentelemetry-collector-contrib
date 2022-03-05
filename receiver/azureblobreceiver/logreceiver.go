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

package azureblobreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

type LogsConsumer interface {
	ConsumeLogsJson(ctx context.Context, json []byte) error
}

type logRceiver struct {
	blobEventHandler BlobEventHandler
	logger           *zap.Logger
	logsUnmarshaler  pdata.LogsUnmarshaler
	nextConsumer     consumer.Logs
	obsrecv          *obsreport.Receiver
}

func (l *logRceiver) Start(ctx context.Context, host component.Host) error {
	l.blobEventHandler.SetLogsConsumer(l)

	l.blobEventHandler.Run(ctx)

	return nil
}

func (l *logRceiver) Shutdown(ctx context.Context) error {
	l.blobEventHandler.Close(ctx)

	return nil
}

func (l *logRceiver) ConsumeLogsJson(ctx context.Context, json []byte) error {
	// l.logger.Info("========logs===========")
	// l.logger.Info(string(json))
	logsContext := l.obsrecv.StartLogsOp(ctx)

	logs, err := l.logsUnmarshaler.UnmarshalLogs(json)
	if err == nil {
		err = l.nextConsumer.ConsumeLogs(logsContext, logs)
	} else {
		l.logger.Error(err.Error())
	}

	l.obsrecv.EndLogsOp(logsContext, typeStr, 1, err)

	return err
}

// Returns a new instance of the log receiver
func NewLogsReceiver(config Config, set component.ReceiverCreateSettings, nextConsumer consumer.Logs, blobEventHandler BlobEventHandler) (component.LogsReceiver, error) {
	logRceiver := &logRceiver{
		blobEventHandler: blobEventHandler,
		logger:           set.Logger,
		logsUnmarshaler:  otlp.NewJSONLogsUnmarshaler(),
		nextConsumer:     nextConsumer,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              "event",
			ReceiverCreateSettings: set,
		}),
	}

	return logRceiver, nil
}
