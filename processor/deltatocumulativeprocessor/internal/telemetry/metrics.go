// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

type Streams struct {
	tracked metric.Int64UpDownCounter
	limit   metric.Int64ObservableGauge
	evicted metric.Int64Counter
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

func (m Metrics) WithLimit(meter metric.Meter, max int64) {
	then := metric.Callback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(m.streams.limit, max)
		return nil
	})
	_, err := meter.RegisterCallback(then, m.streams.limit)
	if err != nil {
		panic(err)
	}
}

func Observe[T any](items streams.Map[T], meter metric.Meter) *Map[T] {
	return &Map[T]{
		Map:     items,
		Metrics: metrics(meter),
	}
}

var _ streams.Map[any] = (*Map[any])(nil)

type Map[T any] struct {
	streams.Map[T]
	Metrics
}

func (m *Map[T]) Store(id streams.Ident, v T) error {
	inc(m.dps.total)
	_, old := m.Load(id)

	var (
		olderStart delta.ErrOlderStart
		outOfOrder delta.ErrOutOfOrder
		gap        delta.ErrGap
		limit      streams.ErrLimit
		evict      streams.ErrEvicted
	)

	err := m.Map.Store(id, v)
	switch {
	case err == nil:
		// all good
	case errors.As(err, &olderStart):
		// non fatal. record but ignore
		inc(m.dps.dropped, reason("older-start"))
		err = nil
	case errors.As(err, &outOfOrder):
		// non fatal. record but ignore
		inc(m.dps.dropped, reason("out-of-order"))
		err = nil
	case errors.As(err, &evict):
		inc(m.streams.evicted)
		err = nil
	case errors.As(err, &limit):
		inc(m.dps.dropped, reason("stream-limit"))
		err = nil
	case errors.As(err, &gap):
		// a gap occurred. record its length, but ignore
		from := gap.From.AsTime()
		to := gap.To.AsTime()
		lost := to.Sub(from).Seconds()
		m.gaps.Add(context.TODO(), int64(lost))
		err = nil
	}

	// not dropped and not seen before => new stream
	if err == nil && !old {
		inc(m.streams.tracked)
	}
	return err
}

func (m *Map[T]) Delete(id streams.Ident) {
	dec(m.streams.tracked)
	m.Map.Delete(id)
}

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
