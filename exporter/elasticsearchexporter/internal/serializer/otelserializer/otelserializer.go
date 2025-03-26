// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"fmt"

	"github.com/elastic/go-freelru"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"
)

type Serializer struct {
	// Data cache for profiles
	knownTraces      *lru.LRUSet[string]
	knownFrames      *lru.LRUSet[string]
	knownExecutables *lru.LRUSet[string]
}

// New builds a new Serializer
func New() (*Serializer, error) {
	// Create LRUs with MinILMRolloverTime as lifetime to avoid losing data by ILM roll-over.
	knownTraces, err := freelru.New[string, lru.Void](knownTracesCacheSize, stringHashFn)
	if err != nil {
		return nil, fmt.Errorf("failed to create traces LRU: %w", err)
	}
	knownTraces.SetLifetime(minILMRolloverTime)

	knownFrames, err := freelru.New[string, lru.Void](knownFramesCacheSize, stringHashFn)
	if err != nil {
		return nil, fmt.Errorf("failed to create frames LRU: %w", err)
	}
	knownFrames.SetLifetime(minILMRolloverTime)

	knownExecutables, err := freelru.New[string, lru.Void](knownExecutablesCacheSize, stringHashFn)
	if err != nil {
		return nil, fmt.Errorf("failed to create executables LRU: %w", err)
	}
	knownExecutables.SetLifetime(minILMRolloverTime)

	return &Serializer{
		knownTraces:      lru.NewLRUSet(knownTraces),
		knownFrames:      lru.NewLRUSet(knownFrames),
		knownExecutables: lru.NewLRUSet(knownExecutables),
	}, nil
}
