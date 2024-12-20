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
	inc(i.dps.total)

	_, old := i.Map.Load(id)
	err := i.Map.Store(id, v)
	if err == nil && !old {
		inc(i.streams.tracked)
	}

	return err
}

func (i Items[T]) Delete(id streams.Ident) {
	dec(i.streams.tracked)
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
		inc(f.dps.dropped, reason("older-start"))
		return streams.Drop
	case errors.As(err, &outOfOrder):
		inc(f.dps.dropped, reason("out-of-order"))
		return streams.Drop
	case errors.As(err, &limit):
		inc(f.dps.dropped, reason("stream-limit"))
		// no space to store stream, drop it instead of failing silently
		return streams.Drop
	case errors.As(err, &evict):
		inc(f.streams.evicted)
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

type addable[Opts any] interface {
	Add(context.Context, int64, ...Opts)
}

func inc[A addable[O], O any](a A, opts ...O) {
	a.Add(context.Background(), 1, opts...)
}

func dec[A addable[O], O any](a A, opts ...O) {
	a.Add(context.Background(), -1, opts...)
}

func reason(reason string) metric.AddOption {
	return metric.WithAttributes(attribute.String("reason", reason))
}
