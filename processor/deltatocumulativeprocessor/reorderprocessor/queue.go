// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reorderprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/gmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/heap"
)

type Queue[T gmetric.Point[T], S gmetric.Slice[S, T]] map[identity.Stream]*heap.Heap[T]

func (q Queue[T, S]) store(id identity.Stream, dp T) {
	if _, ok := q[id]; !ok {
		q[id] = heap.New(func(a, b T) bool { return a.Timestamp() < b.Timestamp() })
	}
	q[id].Push(dp)
}

func (q Queue[T, S]) take(until pcommon.Timestamp) map[identity.Stream]S {
	n := 0
	for _, dps := range q {
		if dps.Len() > 0 && dps.Peek().Timestamp() < until {
			n++
		}
	}

	ms := make(map[identity.Stream]S, n)
	for id, dps := range q {
		s := gmetric.Empty[T, S]()
		for dps.Len() > 0 {
			if dps.Peek().Timestamp() > until {
				break
			}
			dps.Pop().MoveTo(s.AppendEmpty())
		}

		if s.Len() > 0 {
			s.MoveAndAppendTo(ms[id])
		}
	}

	return ms
}
