// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package identity // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"

import (
	"fmt"
	"hash"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata/xhash"
)

type scope = Scope

type Scope struct {
	resource resource

	name      string
	version   string
	schemaURL string
	attrs     [16]byte
}

func (s Scope) Hash() hash.Hash64 {
	sum := s.resource.Hash()
	sum.Write([]byte(s.name))
	sum.Write([]byte(s.version))
	sum.Write([]byte(s.schemaURL))
	sum.Write(s.attrs[:])
	return sum
}

func (s Scope) Resource() Resource {
	return s.resource
}

func (s Scope) String() string {
	return fmt.Sprintf("scope/%x", s.Hash().Sum64())
}

func OfScope(res Resource, scope pcommon.InstrumentationScope) Scope {
	return OfScopeWithSchema(res, scope, "")
}

func OfScopeWithSchema(res Resource, scope pcommon.InstrumentationScope, schemaURL string) Scope {
	return Scope{
		resource:  res,
		name:      scope.Name(),
		version:   scope.Version(),
		schemaURL: schemaURL,
		attrs:     xhash.MapHash(scope.Attributes()),
	}
}
