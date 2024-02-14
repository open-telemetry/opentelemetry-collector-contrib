// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streams // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/streams"

import (
	"hash"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

type Aggregator[D data.Point[D]] interface {
	Aggregate(Ident, D) (D, error)
}

type Ident struct {
	metric metrics.Ident
	attrs  [16]byte
}

func (i Ident) Hash() hash.Hash64 {
	sum := i.metric.Hash()
	sum.Write(i.attrs[:])
	return sum
}

func (i Ident) String() string {
	return strconv.FormatUint(i.Hash().Sum64(), 16)
}

func (i Ident) Metric() metrics.Ident {
	return i.metric
}
