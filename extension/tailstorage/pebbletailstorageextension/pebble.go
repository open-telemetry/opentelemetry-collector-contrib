// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"bytes"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/bloom"
	"go.uber.org/zap"
)

func newPebbleDB(storageDir string, logger *zap.Logger) (*pebble.DB, error) {
	cache := pebble.NewCache(16 << 20)
	defer cache.Unref()

	opts := &pebble.Options{
		// TODO: support persistence across restarts as storage schema matures.
		// Meanwhile, error if DB already exists to prevent users from relying on persistence.
		ErrorIfExists: true,

		FormatMajorVersion: pebble.FormatValueSeparation,
		Logger:             logger.Sugar(),
		MemTableSize:       32 << 20,
		Comparer:           traceKeyComparer(),
		Cache:              cache,
		CompactionConcurrencyRange: func() (lower, upper int) {
			return 2, 4
		},
	}
	opts.Experimental.L0CompactionConcurrency = 4

	opts.Levels[0] = pebble.LevelOptions{
		BlockSize:    32 << 10,
		FilterPolicy: bloom.FilterPolicy(10),
		FilterType:   pebble.TableFilter,
	}

	return pebble.Open(storageDir, opts)
}

func traceKeyComparer() *pebble.Comparer {
	comparer := *pebble.DefaultComparer
	comparer.Split = func(k []byte) int {
		// Since trace ID is fixed sized, and trace ID separator is a valid trace ID byte itself,
		// split on fixed length instead to avoid corruption.
		if len(k) >= traceIDBytes+1 {
			return traceIDBytes + 1
		}
		return len(k) // BUG: key len should always be >= 17
	}
	comparer.Compare = func(a, b []byte) int {
		ap := comparer.Split(a)
		bp := comparer.Split(b)
		if prefixCmp := bytes.Compare(a[:ap], b[:bp]); prefixCmp != 0 {
			return prefixCmp
		}
		return comparer.ComparePointSuffixes(a[ap:], b[bp:])
	}
	comparer.Name = "tailsampling.TailStorageComparer"
	return &comparer
}
