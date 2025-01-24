// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cache"

import "go.opentelemetry.io/collector/pdata/pcommon"

// Cache is a cache using a pcommon.TraceID as the key and any generic type as the value.
type Cache[V any] interface {
	// Get returns the value for the given id, and a boolean to indicate whether the key was found.
	// If the key is not present, the zero value is returned.
	Get(id pcommon.TraceID) (V, bool)
	// Put sets the value for a given id
	Put(id pcommon.TraceID, v V) bool
	// Delete deletes the value for the given id
	Delete(id pcommon.TraceID)
}
