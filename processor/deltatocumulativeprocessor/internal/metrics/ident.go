// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"hash"
	"hash/fnv"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Ident struct {
	ScopeIdent

	name string
	unit string
	ty   string

	monotonic   bool
	temporality pmetric.AggregationTemporality
}

func (i Ident) Hash() hash.Hash64 {
	sum := i.ScopeIdent.Hash()
	sum.Write([]byte(i.name))
	sum.Write([]byte(i.unit))
	sum.Write([]byte(i.ty))

	var mono byte
	if i.monotonic {
		mono = 1
	}
	sum.Write([]byte{mono, byte(i.temporality)})
	return sum
}

type ScopeIdent struct {
	ResourceIdent

	name    string
	version string
	attrs   [16]byte
}

func (s ScopeIdent) Hash() hash.Hash64 {
	sum := s.ResourceIdent.Hash()
	sum.Write([]byte(s.name))
	sum.Write([]byte(s.version))
	sum.Write(s.attrs[:])
	return sum
}

type ResourceIdent struct {
	attrs [16]byte
}

func (r ResourceIdent) Hash() hash.Hash64 {
	sum := fnv.New64a()
	sum.Write(r.attrs[:])
	return sum
}
