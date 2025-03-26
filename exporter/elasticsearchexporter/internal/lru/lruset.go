// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lru // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"

import (
	"github.com/elastic/go-freelru"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
)

type Void struct{}

// LockedLRUSet is the interface provided to the LRUSet once a lock has been
// acquired.
type LockedLRUSet[T comparable] interface {
	// CheckAndAdd checks whether the entry is already stored in the cache, and
	// adds it.
	// It returns whether the entry should be excluded, as it was already present
	// in cache.
	CheckAndAdd(entry T) bool
}

// LRUSet is an LRU cache implementation that allows acquiring a lock, and
// checking whether specific keys have already been stored.
type LRUSet[T comparable] struct {
	syncMu *xsync.RWMutex[*freelru.LRU[T, Void]]
}

func (l *LRUSet[T]) WithLock(fn func(LockedLRUSet[T]) error) error {
	if l == nil || l.syncMu == nil {
		return fn(nilLockedLRUSet[T]{})
	}

	excluded := l.syncMu.WLock()
	defer l.syncMu.WUnlock(&excluded)

	return fn(lockedLRUSet[T]{*excluded})
}

func NewLRUSet[T comparable](lru *freelru.LRU[T, Void]) *LRUSet[T] {
	syncMu := xsync.NewRWMutex(lru)
	return &LRUSet[T]{syncMu: &syncMu}
}

type lockedLRUSet[T comparable] struct {
	excluded *freelru.LRU[T, Void]
}

func (l lockedLRUSet[T]) CheckAndAdd(entry T) bool {
	if _, exclude := (l.excluded).Get(entry); exclude {
		return true
	}
	defer (l.excluded).Add(entry, Void{})
	return false
}

type nilLockedLRUSet[T comparable] struct{}

func (l nilLockedLRUSet[T]) CheckAndAdd(T) bool {
	return false
}
