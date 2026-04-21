// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Cache is a cache implementation for the tailsamplingprocessor's decision cache.
type Cache interface {
	// Get returns the decision for the given id, and a boolean to indicate whether the key was found.
	// Returning a zero value for DecisionMetadata is valid for caches that do not store metadata.
	// Callers should check the boolean return value to determine if the key was found.
	Get(id pcommon.TraceID) (DecisionMetadata, bool)
	// Put sets the decision for a given id. If the key is already present, the decision is overwritten.
	Put(id pcommon.TraceID, metadata DecisionMetadata)
}

type DecisionMetadata struct {
	PolicyName string
}
