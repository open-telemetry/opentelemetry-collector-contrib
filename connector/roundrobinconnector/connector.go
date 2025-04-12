// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package roundrobinconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/roundrobinconnector"

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

func allConsumers[T any](r router[T]) ([]T, error) {
	pipeIDs := r.PipelineIDs()
	consumers := make([]T, len(pipeIDs))
	for i, pipeID := range pipeIDs {
		cons, err := r.Consumer(pipeID)
		if err != nil {
			return nil, err
		}
		consumers[i] = cons
	}
	return consumers, nil
}

type router[T any] interface {
	PipelineIDs() []pipeline.ID
	Consumer(pipelineIDs ...pipeline.ID) (T, error)
}

func newLogs(nextConsumer consumer.Logs) (connector.Logs, error) {
	nextConsumers, err := allConsumers[consumer.Logs](nextConsumer.(connector.LogsRouterAndConsumer))
	if err != nil {
		return nil, err
	}
	return &roundRobin{nextLogs: nextConsumers}, nil
}

func newMetrics(nextConsumer consumer.Metrics) (connector.Metrics, error) {
	nextConsumers, err := allConsumers[consumer.Metrics](nextConsumer.(connector.MetricsRouterAndConsumer))
	if err != nil {
		return nil, err
	}
	return &roundRobin{nextMetrics: nextConsumers}, nil
}

func newTraces(nextConsumer consumer.Traces) (connector.Traces, error) {
	nextConsumers, err := allConsumers[consumer.Traces](nextConsumer.(connector.TracesRouterAndConsumer))
	if err != nil {
		return nil, err
	}
	return &roundRobin{nextTraces: nextConsumers}, nil
}

// roundRobin is used to pass signals directly from one pipeline to one of the configured once in a round-robin mode.
// This is useful when there is a need to scale (shard) data processing and downstream components do not
// handle concurrent requests very well.
type roundRobin struct {
	component.StartFunc
	component.ShutdownFunc
	nextConsumer atomic.Uint64
	nextMetrics  []consumer.Metrics
	nextLogs     []consumer.Logs
	nextTraces   []consumer.Traces
}

func (rr *roundRobin) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (rr *roundRobin) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return rr.nextLogs[rr.nextConsumer.Add(1)%uint64(len(rr.nextLogs))].ConsumeLogs(ctx, ld)
}

func (rr *roundRobin) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return rr.nextMetrics[rr.nextConsumer.Add(1)%uint64(len(rr.nextMetrics))].ConsumeMetrics(ctx, md)
}

func (rr *roundRobin) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return rr.nextTraces[rr.nextConsumer.Add(1)%uint64(len(rr.nextTraces))].ConsumeTraces(ctx, td)
}
