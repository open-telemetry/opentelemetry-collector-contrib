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

	// write until limit must work
	for i := 0; i < 10; i++ {
		id, dp := sum.Stream()
		ids[i] = id
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

	// after removing one, must be accepted again
	{
		lim.Delete(ids[0])

		id, dp := sum.Stream()
		err := lim.Store(id, dp)
		require.NoError(t, err)
	}
}
