// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lru // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/lru"

import (
	"time"

	"github.com/cespare/xxhash"
	"github.com/elastic/go-freelru"
	"go.opentelemetry.io/ebpf-profiler/libpf/xsync"
)

type void struct{}

func stringHashFn(s string) uint32 {
	return uint32(xxhash.Sum64String(s))
}

// LockedLRUSet is the interface provided to the LRUSet once a lock has been
// acquired.
type LockedLRUSet interface {
	// CheckAndAdd checks whether the entry is already stored in the cache, and
	// adds it.
	// It returns whether the entry should be excluded, as it was already present
	// in cache.
	CheckAndAdd(entry string) bool
}

// LRUSet is an LRU cache implementation that allows acquiring a lock, and
// checking whether specific keys have already been stored.
type LRUSet struct {
	syncMu *xsync.RWMutex[*freelru.LRU[string, void]]
}

func (l *LRUSet) WithLock(fn func(LockedLRUSet) error) error {
	lru := l.syncMu.WLock()
	defer l.syncMu.WUnlock(&lru)

	return fn(lockedLRUSet{*lru})
}

func NewLRUSet(size uint32, rollover time.Duration) (*LRUSet, error) {
	lru, err := freelru.New[string, void](size, stringHashFn)
	if err != nil {
		return nil, err
	}
	lru.SetLifetime(rollover)

	syncMu := xsync.NewRWMutex(lru)
	return &LRUSet{syncMu: &syncMu}, nil
}

type lockedLRUSet struct {
	lru *freelru.LRU[string, void]
}

func (l lockedLRUSet) CheckAndAdd(entry string) (excluded bool) {
	if _, exclude := (l.lru).Get(entry); exclude {
		return true
	}
	(l.lru).Add(entry, void{})
	return false
}
