// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dedup // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/dedup"

import (
	"sync"
	"time"
)

// Simple fingerprint-based deduper with TTL.
type FingerprintDeduper struct {
	mu   sync.Mutex
	data map[uint64]time.Time
	ttl  time.Duration
}

func New(ttl time.Duration) *FingerprintDeduper {
	return &FingerprintDeduper{
		data: make(map[uint64]time.Time),
		ttl:  ttl,
	}
}

func (d *FingerprintDeduper) Seen(fp uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if t, ok := d.data[fp]; ok && now.Sub(t) < d.ttl {
		return true
	}
	d.data[fp] = now
	// lazy GC
	for k, t := range d.data {
		if now.Sub(t) >= d.ttl {
			delete(d.data, k)
		}
	}
	return false
}
