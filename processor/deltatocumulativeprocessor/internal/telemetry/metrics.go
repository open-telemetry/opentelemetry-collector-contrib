// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

type Telemetry struct {
	Metrics

	meter metric.Meter
}

func New(meter metric.Meter) Telemetry {
	return Telemetry{
		Metrics: metrics(meter),
		meter:   meter,
	}
}

type Streams struct {
	tracked metric.Int64UpDownCounter
	limit   metric.Int64ObservableGauge
	evicted metric.Int64Counter
	stale   metric.Int64ObservableGauge
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

func metrics(meter metric.Meter) Metrics {
	var (
		count  = use(meter.Int64Counter)
		updown = use(meter.Int64UpDownCounter)
		gauge  = use(meter.Int64ObservableGauge)
	)

	return Metrics{
		streams: Streams{
			tracked: updown("streams.tracked",
				metric.WithDescription("number of streams tracked"),
				metric.WithUnit("{stream}"),
			),
			limit: gauge("streams.limit",
				metric.WithDescription("upper limit of tracked streams"),
				metric.WithUnit("{stream}"),
			),
			evicted: count("streams.evicted",
				metric.WithDescription("number of streams evicted"),
				metric.WithUnit("{stream}"),
			),
			stale: gauge("streams.max_stale",
				metric.WithDescription("duration without new samples after which streams are dropped"),
				metric.WithUnit("s"),
			),
		},
		dps: Datapoints{
			total: count("datapoints.processed",
				metric.WithDescription("number of datapoints processed"),
				metric.WithUnit("{datapoint}"),
			),
			dropped: count("datapoints.dropped",
				metric.WithDescription("number of dropped datapoints due to given 'reason'"),
				metric.WithUnit("{datapoint}"),
			),
		},
		gaps: count("gaps.length",
			metric.WithDescription("total duration where data was expected but not received"),
			metric.WithUnit("s"),
		),
	}
}

func (tel Telemetry) WithLimit(max int64) {
	then := metric.Callback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(tel.streams.limit, max)
		return nil
	})
	_, err := tel.meter.RegisterCallback(then, tel.streams.limit)
	if err != nil {
		panic(err)
	}
}

func (tel Telemetry) WithStale(max time.Duration) {
	then := metric.Callback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(tel.streams.stale, int64(max.Seconds()))
		return nil
	})
	_, err := tel.meter.RegisterCallback(then, tel.streams.stale)
	if err != nil {
		panic(err)
	}
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
	case errors.As(err, &outOfOrder):
		inc(f.dps.dropped, reason("out-of-order"))
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

func use[F func(string, ...O) (M, error), M any, O any](f F) func(string, ...O) M {
	return func(name string, opts ...O) M {
		name = processorhelper.BuildCustomMetricName(metadata.Type.String(), name)
		m, err := f(name, opts...)
		if err != nil {
			panic(err)
		}
		return m
	}
}
