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

type TracesConsumer interface {
	ConsumeTracesJson(ctx context.Context, json []byte) error
}

type traceRceiver struct {
	blobEventHandler  BlobEventHandler
	logger            *zap.Logger
	tracesUnmarshaler pdata.TracesUnmarshaler
	nextConsumer      consumer.Traces
	obsrecv           *obsreport.Receiver
}

func (t *traceRceiver) Start(ctx context.Context, host component.Host) error {
	t.blobEventHandler.SetTracesConsumer(t)

	t.blobEventHandler.Run(ctx)

	return nil
}

func (t *traceRceiver) Shutdown(ctx context.Context) error {
	t.blobEventHandler.Close(ctx)

	return nil
}

func (t *traceRceiver) ConsumeTracesJson(ctx context.Context, json []byte) error {
	tracesContext := t.obsrecv.StartTracesOp(ctx)

	traces, err := t.tracesUnmarshaler.UnmarshalTraces(json)
	if err == nil {
		err = t.nextConsumer.ConsumeTraces(tracesContext, traces)
	} else {
		t.logger.Error(err.Error())
	}

	t.obsrecv.EndTracesOp(tracesContext, typeStr, 1, err)

	return err
}

// Returns a new instance of the traces receiver
func NewTraceReceiver(config Config, set component.ReceiverCreateSettings, nextConsumer consumer.Traces, blobEventHandler BlobEventHandler) (component.TracesReceiver, error) {
	traceRceiver := &traceRceiver{
		blobEventHandler:  blobEventHandler,
		logger:            set.Logger,
		tracesUnmarshaler: otlp.NewJSONTracesUnmarshaler(),
		nextConsumer:      nextConsumer,
		obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             config.ID(),
			Transport:              "event",
			ReceiverCreateSettings: set,
		}),
	}

	return traceRceiver, nil
}
