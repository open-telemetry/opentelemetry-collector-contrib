// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

// TestFaults verifies certain non-fatal errors are actually caused and
// subsequently dropped. It does so by writing bad samples to the actual
// implementation instead of fabricating errors manually.
func TestFaults(t *testing.T) {
	type Map = streams.Map[data.Number]
	type Case struct {
		Name string
		Map  Map
		Pre  func(Map, identity.Stream, data.Number) error
		Bad  func(Map, identity.Stream, data.Number) error
		Err  error
		Want error
	}

	sum := random.Sum()
	evid, evdp := sum.Stream()

	cases := []Case{
		{
			Name: "older-start",
			Pre: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetStartTimestamp(ts(20))
				dp.SetTimestamp(ts(30))
				return dps.Store(id, dp)
			},
			Bad: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetStartTimestamp(ts(10))
				dp.SetTimestamp(ts(40))
				return dps.Store(id, dp)
			},
			Err: delta.ErrOlderStart{Start: ts(20), Sample: ts(10)},
		},
		{
			Name: "out-of-order",
			Pre: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetTimestamp(ts(20))
				return dps.Store(id, dp)
			},
			Bad: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetTimestamp(ts(10))
				return dps.Store(id, dp)
			},
			Err: delta.ErrOutOfOrder{Last: ts(20), Sample: ts(10)},
		},
		{
			Name: "gap",
			Pre: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetStartTimestamp(ts(10))
				dp.SetTimestamp(ts(20))
				return dps.Store(id, dp)
			},
			Bad: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetStartTimestamp(ts(30))
				dp.SetTimestamp(ts(40))
				return dps.Store(id, dp)
			},
			Err: delta.ErrGap{From: ts(20), To: ts(30)},
		},
		{
			Name: "limit",
			Map:  streams.Limit(delta.New[data.Number](), 1),
			Pre: func(dps Map, id identity.Stream, dp data.Number) error {
				dp.SetTimestamp(ts(10))
				return dps.Store(id, dp)
			},
			Bad: func(dps Map, _ identity.Stream, _ data.Number) error {
				id, dp := sum.Stream()
				dp.SetTimestamp(ts(20))
				return dps.Store(id, dp)
			},
			Err:  streams.ErrLimit(1),
			Want: streams.Drop, // we can't ignore being at limit, we need to drop the entire stream for this request
		},
		{
			Name: "evict",
			Map: func() Map {
				ev := HeadEvictor[data.Number]{Map: delta.New[data.Number]()}
				lim := streams.Limit(ev, 1)
				lim.Evictor = ev
				return lim
			}(),
			Pre: func(dps Map, _ identity.Stream, _ data.Number) error {
				evdp.SetTimestamp(ts(10))
				return dps.Store(evid, evdp)
			},
			Bad: func(dps Map, _ identity.Stream, _ data.Number) error {
				id, dp := sum.Stream()
				dp.SetTimestamp(ts(20))
				return dps.Store(id, dp)
			},
			Err: streams.ErrEvicted{Ident: evid, ErrLimit: streams.ErrLimit(1)},
		},
	}

	telb, err := metadata.NewTelemetryBuilder(component.TelemetrySettings{MeterProvider: noop.NewMeterProvider()})
	require.NoError(t, err)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			id, dp := sum.Stream()
			tel := telemetry.New(telb)

			dps := c.Map
			if dps == nil {
				dps = delta.New[data.Number]()
			}
			onf := telemetry.ObserveNonFatal(dps, &tel.Metrics)

			if c.Pre != nil {
				err := c.Pre(onf, id, dp.Clone())
				require.NoError(t, err)
			}

			err := c.Bad(dps, id, dp.Clone())
			require.Equal(t, c.Err, err)

			err = c.Bad(onf, id, dp.Clone())
			require.Equal(t, c.Want, err)
		})
	}
}

type ts = pcommon.Timestamp

// HeadEvictor drops the first stream on Evict()
type HeadEvictor[T any] struct{ streams.Map[T] }

func (e HeadEvictor[T]) Evict() (evicted identity.Stream, ok bool) {
	e.Items()(func(id identity.Stream, _ T) bool {
		e.Delete(id)
		evicted = id
		return false
	})
	return evicted, true
}
