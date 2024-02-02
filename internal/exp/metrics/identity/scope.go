// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"hash"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Scope struct {
	res Resource

	name    string
	version string
	attrs   [16]byte
}

func (s Scope) Hash() hash.Hash64 {
	sum := s.res.Hash()
	sum.Write([]byte(s.name))
	sum.Write([]byte(s.version))
	sum.Write(s.attrs[:])
	return sum
}

func OfScope(res Resource, scope pcommon.InstrumentationScope) Scope {
	return Scope{
		res:     res,
		name:    scope.Name(),
		version: scope.Version(),
		attrs:   pdatautil.MapHash(scope.Attributes()),
	}
}
