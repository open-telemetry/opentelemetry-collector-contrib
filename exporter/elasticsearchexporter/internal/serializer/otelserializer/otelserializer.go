// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"
)

const (
	KiB = 1024
	MiB = 1024 * KiB

	knownExecutablesCacheSize = 128 * KiB
	knownFramesCacheSize      = 128 * KiB
	knownTracesCacheSize      = 128 * KiB

	minILMRolloverTime = 3 * time.Hour
)

type Serializer struct {
	// Data cache for profiles
	knownTraces      *lru.LRUSet
	knownFrames      *lru.LRUSet
	knownExecutables *lru.LRUSet
}

// New builds a new Serializer
func New() (*Serializer, error) {
	// Create LRUs with MinILMRolloverTime as lifetime to avoid losing data by ILM roll-over.
	knownTraces, err := lru.NewLRUSet(knownTracesCacheSize, minILMRolloverTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create traces LRU: %w", err)
	}

	knownFrames, err := lru.NewLRUSet(knownFramesCacheSize, minILMRolloverTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create frames LRU: %w", err)
	}

	knownExecutables, err := lru.NewLRUSet(knownExecutablesCacheSize, minILMRolloverTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create executables LRU: %w", err)
	}

	return &Serializer{
		knownTraces:      knownTraces,
		knownFrames:      knownFrames,
		knownExecutables: knownExecutables,
	}, nil
}
