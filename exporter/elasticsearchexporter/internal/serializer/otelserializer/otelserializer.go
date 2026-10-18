// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"
)

const (
	knownExecutablesCacheSize = 16 * 1024
	knownFramesCacheSize      = 128 * 1024
	knownTracesCacheSize      = 128 * 1024
	knownHostsCacheSize       = 1024 * 1024

	// knownDocsRefreshInterval bounds how long a deduplicated document (stack trace,
	// stack frame, executable, host) goes without being written again. Data streams
	// only support create, so each refresh appends a copy to the current backing
	// index; this keeps documents still in use alive across the 60d data retention
	// while old backing indices get deleted.
	knownDocsRefreshInterval = 24 * time.Hour
)

type Serializer struct {
	// Data cache for profiles
	loadLRUsOnce     sync.Once
	lruErr           error
	knownTraces      *lru.LRUSet
	knownFrames      *lru.LRUSet
	knownExecutables *lru.LRUSet
	knownHosts       *lru.LRUSet
}

// New builds a new Serializer
func New() (*Serializer, error) {
	return &Serializer{}, nil
}
