// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"
)

const (
	knownExecutablesCacheSize             = 16 * 1024
	knownFramesCacheSize                  = 128 * 1024
	knownTracesCacheSize                  = 128 * 1024
	knownUnsymbolizedFramesCacheSize      = 128 * 1024
	knownUnsymbolizedExecutablesCacheSize = 16 * 1024

	minILMRolloverTime = 3 * time.Hour
)

type Serializer struct {
	// Data cache for profiles
	loadLRUsOnce                 sync.Once
	lruErr                       error
	knownTraces                  *lru.LRUSet
	knownFrames                  *lru.LRUSet
	knownExecutables             *lru.LRUSet
	knownUnsymbolizedFrames      *lru.LRUSet
	knownUnsymbolizedExecutables *lru.LRUSet
}

// New builds a new Serializer
func New() (*Serializer, error) {
	return &Serializer{}, nil
}
