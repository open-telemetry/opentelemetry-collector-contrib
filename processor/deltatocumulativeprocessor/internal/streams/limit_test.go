// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	exp "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testdata/random"
)

func TestLimit(t *testing.T) {
	sum := random.Sum()

	items := make(exp.HashMap[data.Number])
	lim := streams.Limit(items, 10)

	ids := make([]identity.Stream, 10)
	dps := make([]data.Number, 10)

	// write until limit must work
	for i := 0; i < 10; i++ {
		id, dp := sum.Stream()
		ids[i] = id
		dps[i] = dp
		err := lim.Store(id, dp)
		require.NoError(t, err)
	}

	// one over limit must be rejected
	{
		id, dp := sum.Stream()
		err := lim.Store(id, dp)
		want := streams.ErrLimit(10)
		require.ErrorAs(t, err, &want)
		require.True(t, streams.AtLimit(err))
	}

	// write to existing must work
	{
		err := lim.Store(ids[3], dps[3])
		require.NoError(t, err)
	}

	// after removing one, must be accepted again
	{
		lim.Delete(ids[0])

		id, dp := sum.Stream()
		err := lim.Store(id, dp)
		require.NoError(t, err)
	}
}

func TestLimitEvict(t *testing.T) {
	sum := random.Sum()
	evictable := make(map[identity.Stream]struct{})

	items := make(exp.HashMap[data.Number])
	lim := streams.Limit(items, 5)

	ids := make([]identity.Stream, 10)
	lim.Evictor = streams.EvictorFunc(func() (identity.Stream, bool) {
		for _, id := range ids {
			if _, ok := evictable[id]; ok {
				delete(evictable, id)
				return id, true
			}
		}
		return identity.Stream{}, false
	})

	dps := make([]data.Number, 10)
	for i := 0; i < 10; i++ {
		id, dp := sum.Stream()
		ids[i] = id
		dps[i] = dp
	}

	// store up to limit must work
	for i := 0; i < 5; i++ {
		err := lim.Store(ids[i], dps[i])
		require.NoError(t, err)
	}

	// store beyond limit must fail
	for i := 5; i < 10; i++ {
		err := lim.Store(ids[i], dps[i])
		require.Equal(t, streams.ErrLimit(5), err)
	}

	// put two streams up for eviction
	evictable[ids[2]] = struct{}{}
	evictable[ids[3]] = struct{}{}

	// while evictable do so, fail again afterwards
	for i := 5; i < 10; i++ {
		err := lim.Store(ids[i], dps[i])
		if i < 7 {
			require.Equal(t, streams.ErrEvicted{ErrLimit: streams.ErrLimit(5), Ident: ids[i-3]}, err)
		} else {
			require.Equal(t, streams.ErrLimit(5), err)
		}
	}
}
