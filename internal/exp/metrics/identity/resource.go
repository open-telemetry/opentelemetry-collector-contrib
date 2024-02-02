// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"hash"
	"hash/fnv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Resource struct {
	attrs [16]byte
}

func (r Resource) Hash() hash.Hash64 {
	sum := fnv.New64a()
	sum.Write(r.attrs[:])
	return sum
}

func OfResource(r pcommon.Resource) Resource {
	return Resource{
		attrs: pdatautil.MapHash(r.Attributes()),
	}
}
