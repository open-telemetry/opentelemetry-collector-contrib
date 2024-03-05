package telemetry

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	streams metric.Int64UpDownCounter
	total   metric.Int64Counter
	dropped metric.Int64Counter
	lost    metric.Int64Counter
}

func metrics(meter metric.Meter) Metrics {
	var (
		count  = use(meter.Int64Counter)
		updown = use(meter.Int64UpDownCounter)
	)

	return Metrics{
		streams: updown("streams",
			metric.WithDescription("number of streams tracked"),
			metric.WithUnit("{stream}"),
		),
		total: count("datapoints.processed",
			metric.WithDescription("number of datapoints processed"),
			metric.WithUnit("{datapoint}"),
		),
		dropped: count("datapoints.dropped",
			metric.WithDescription("number of dropped datapoints due to given 'reason'"),
			metric.WithUnit("{datapoint}"),
		),
		lost: count("lost",
			metric.WithDescription("total duration where data was expected but not received"),
			metric.WithUnit("s"),
		),
	}
}

func Observe[T any](items streams.Map[T], meter metric.Meter) streams.Map[T] {
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
	inc(m.total)
	_, old := m.Load(id)

	var (
		olderStart *delta.ErrOlderStart
		outOfOrder *delta.ErrOutOfOrder
		gap        *delta.ErrGap
	)

	err := m.Map.Store(id, v)
	switch {
	case err == nil:
		// all good
	case errors.As(err, olderStart):
		// non fatal. record but ignore
		inc(m.dropped, reason(olderStart))
		err = nil
	case errors.As(err, outOfOrder):
		// non fatal. record but ignore
		inc(m.dropped, reason(outOfOrder))
		err = nil
	case errors.As(err, gap):
		// a gap occured. record its length, but ignore
		from := gap.From.AsTime()
		to := gap.To.AsTime()
		lost := to.Sub(from).Seconds()
		m.lost.Add(nil, int64(lost))
		err = nil
	}

	// not dropped and not seen before => new stream
	if err == nil && !old {
		inc(m.streams)
	}
	return err
}

func (m *Map[T]) Delete(id streams.Ident) {
	dec(m.streams)
	m.Map.Delete(id)
}

type addable[Opts any] interface {
	Add(context.Context, int64, ...Opts)
}

func inc[A addable[O], O any](a A, opts ...O) {
	a.Add(nil, 1, opts...)
}

func dec[A addable[O], O any](a A, opts ...O) {
	a.Add(nil, -1, opts...)
}

func reason[E error](err *E) metric.AddOption {
	reason := reflect.TypeOf(*new(E)).Name()
	reason = strings.TrimPrefix(reason, "Err")
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
