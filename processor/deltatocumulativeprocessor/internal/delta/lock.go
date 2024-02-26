// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package delta // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/delta"

import (
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"
)

var _ streams.Aggregator[data.Number] = (*Lock[data.Number])(nil)

type Lock[D data.Point[D]] struct {
	sync.Mutex
	next streams.Aggregator[D]
}

func (l *Lock[D]) Aggregate(id streams.Ident, dp D) (D, error) {
	l.Lock()
	dp, err := l.next.Aggregate(id, dp)
	l.Unlock()
	return dp, err
}
