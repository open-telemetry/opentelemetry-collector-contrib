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
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"golang.org/x/net/websocket"
)

type repeatingMetricsConsumer struct {
	webSocket *websocket.Conn
	marshaler pmetric.Marshaler
	next      consumer.Metrics
	stop      chan struct{}
}

func (*repeatingMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *repeatingMetricsConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	marshaled, err := c.marshaler.MarshalMetrics(md)
	if err != nil {
		panic(err)
	}
	_, err = c.webSocket.Write(marshaled)
	if err != nil {
		fmt.Printf("c.webSocket.Write(marshaled): %v\n", err)
		c.stop <- struct{}{}
		return nil
	}
	if c.next == nil {
		return nil
	}
	return c.next.ConsumeMetrics(ctx, md)
}

type repeatingLogsConsumer struct {
	webSocket *websocket.Conn
	marshaler plog.Marshaler
	next      consumer.Logs
	stop      chan struct{}
}

func (*repeatingLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *repeatingLogsConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	marshaled, err := c.marshaler.MarshalLogs(logs)
	if err != nil {
		panic(err)
	}
	_, err = c.webSocket.Write(marshaled)
	if err != nil {
		fmt.Printf("c.webSocket.Write(marshaled): %v\n", err)
		c.stop <- struct{}{}
		return nil
	}
	if c.next == nil {
		return nil
	}
	return c.next.ConsumeLogs(ctx, logs)
}

type repeatingTracesConsumer struct {
	webSocket *websocket.Conn
	marshaler ptrace.Marshaler
	next      consumer.Traces
	stop      chan struct{}
}

func (c *repeatingTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *repeatingTracesConsumer) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	marshaled, err := c.marshaler.MarshalTraces(traces)
	if err != nil {
		panic(err)
	}
	_, err = c.webSocket.Write(marshaled)
	if err != nil {
		fmt.Printf("c.webSocket.Write(marshaled): %v\n", err)
		c.stop <- struct{}{}
		return nil
	}
	if c.next == nil {
		return nil
	}
	return c.next.ConsumeTraces(ctx, traces)
}
