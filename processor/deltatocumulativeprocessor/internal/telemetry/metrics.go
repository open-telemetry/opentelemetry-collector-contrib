// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

const reason = "reason"

var (
	reasonOlderStart  = metric.WithAttributeSet(attribute.NewSet(attribute.String(reason, "older-start")))
	reasonOutOfOrder  = metric.WithAttributeSet(attribute.NewSet(attribute.String(reason, "out-of-order")))
	reasonStreamLimit = metric.WithAttributeSet(attribute.NewSet(attribute.String(reason, "stream-limit")))
)

type Telemetry struct {
	Metrics
}

func New(telb *metadata.TelemetryBuilder) Telemetry {
	return Telemetry{Metrics: Metrics{
		streams: Streams{
			tracked: telb.DeltatocumulativeStreamsTracked,
			limit:   telb.DeltatocumulativeStreamsLimit,
			evicted: telb.DeltatocumulativeStreamsEvicted,
			stale:   telb.DeltatocumulativeStreamsMaxStale,
		},
		dps: Datapoints{
			total:   telb.DeltatocumulativeDatapointsProcessed,
			dropped: telb.DeltatocumulativeDatapointsDropped,
		},
		gaps: telb.DeltatocumulativeGapsLength,
	}}
}

type Streams struct {
	tracked metric.Int64UpDownCounter
	limit   metric.Int64Gauge
	evicted metric.Int64Counter
	stale   metric.Int64Gauge
}

type Datapoints struct {
	total   metric.Int64Counter
	dropped metric.Int64Counter
}

type Metrics struct {
	streams Streams
	dps     Datapoints

	gaps metric.Int64Counter
}

func (tel Telemetry) WithLimit(max int64) {
	tel.streams.limit.Record(context.Background(), max)
}

func (tel Telemetry) WithStale(max time.Duration) {
	tel.streams.stale.Record(context.Background(), int64(max.Seconds()))
}

func ObserveItems[T any](items streams.Map[T], metrics *Metrics) Items[T] {
	return Items[T]{
		Map:     items,
		Metrics: metrics,
	}
}

func ObserveNonFatal[T any](items streams.Map[T], metrics *Metrics) Faults[T] {
	return Faults[T]{
		Map:     items,
		Metrics: metrics,
	}
}

type Items[T any] struct {
	streams.Map[T]
	*Metrics
}

func (i Items[T]) Store(id streams.Ident, v T) error {
	i.dps.total.Add(context.Background(), 1)

	_, old := i.Map.Load(id)
	err := i.Map.Store(id, v)
	if err == nil && !old {
		i.streams.tracked.Add(context.Background(), 1)
	}

	return err
}

func (i Items[T]) Delete(id streams.Ident) {
	i.streams.tracked.Add(context.TODO(), -1)
	i.Map.Delete(id)
}

type Faults[T any] struct {
	streams.Map[T]
	*Metrics
}

func (f Faults[T]) Store(id streams.Ident, v T) error {
	var (
		olderStart delta.ErrOlderStart
		outOfOrder delta.ErrOutOfOrder
		gap        delta.ErrGap
		limit      streams.ErrLimit
		evict      streams.ErrEvicted
	)

	err := f.Map.Store(id, v)
	switch {
	default:
		return err
	case errors.As(err, &olderStart):
		f.dps.dropped.Add(context.TODO(), 1, reasonOlderStart)
		return streams.Drop
	case errors.As(err, &outOfOrder):
		f.dps.dropped.Add(context.TODO(), 1, reasonOutOfOrder)
		return streams.Drop
	case errors.As(err, &limit):
		f.dps.dropped.Add(context.TODO(), 1, reasonStreamLimit)
		// no space to store stream, drop it instead of failing silently
		return streams.Drop
	case errors.As(err, &evict):
		f.streams.evicted.Add(context.TODO(), 1)
	case errors.As(err, &gap):
		from := gap.From.AsTime()
		to := gap.To.AsTime()
		lost := to.Sub(from).Seconds()
		f.gaps.Add(context.TODO(), int64(lost))
	}

	return nil
}

var (
	_ streams.Map[any] = (*Items[any])(nil)
	_ streams.Map[any] = (*Faults[any])(nil)
)
