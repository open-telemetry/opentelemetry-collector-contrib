// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otto

import (
	"context"
	"encoding/json"
	"log"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"golang.org/x/net/websocket"
)

type metricsRepeater struct {
	logger    *log.Logger
	ws        *websocket.Conn
	marshaler pmetric.Marshaler
	next      consumer.Metrics
	stop      chan struct{}
}

func (*metricsRepeater) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (r *metricsRepeater) ConsumeMetrics(ctx context.Context, pmetrics pmetric.Metrics) error {
	envelopeJson, err := json.Marshal(wsMessageEnvelope{
		Payload: metrics(pmetrics),
	})
	if err != nil {
		r.logger.Printf("error marshaling envelope: %v", err)
		return nil
	}
	_, err = r.ws.Write(envelopeJson)
	if err != nil {
		r.logger.Printf("error writing envelope json to websocket: %v\n", err)
		r.stop <- struct{}{}
		return nil
	}
	if r.next == nil {
		return nil
	}
	return r.next.ConsumeMetrics(ctx, pmetrics)
}

type logsRepeater struct {
	logger    *log.Logger
	ws        *websocket.Conn
	marshaler plog.Marshaler
	next      consumer.Logs
	stop      chan struct{}
}

func (*logsRepeater) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (r *logsRepeater) ConsumeLogs(ctx context.Context, plogs plog.Logs) error {
	envelopeJson, err := json.Marshal(wsMessageEnvelope{
		Payload: logs(plogs),
	})
	if err != nil {
		r.logger.Printf("error marshaling envelope: %v", err)
		return nil
	}
	_, err = r.ws.Write(envelopeJson)
	if err != nil {
		r.logger.Printf("error writing envelope json to websocket: %v\n", err)
		r.stop <- struct{}{}
		return nil
	}
	if r.next == nil {
		return nil
	}
	return r.next.ConsumeLogs(ctx, plogs)
}

type tracesRepeater struct {
	logger    *log.Logger
	ws        *websocket.Conn
	marshaler ptrace.Marshaler
	next      consumer.Traces
	stop      chan struct{}
}

func (r *tracesRepeater) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (r *tracesRepeater) ConsumeTraces(ctx context.Context, ptraces ptrace.Traces) error {
	envelopeJson, err := json.Marshal(wsMessageEnvelope{
		Payload: traces(ptraces),
	})
	if err != nil {
		r.logger.Printf("error marshaling envelope: %v", err)
		return nil
	}
	_, err = r.ws.Write(envelopeJson)
	if err != nil {
		r.logger.Printf("error writing envelope json to websocket: %v\n", err)
		r.stop <- struct{}{}
		return nil
	}
	if r.next == nil {
		return nil
	}
	return r.next.ConsumeTraces(ctx, ptraces)
}
