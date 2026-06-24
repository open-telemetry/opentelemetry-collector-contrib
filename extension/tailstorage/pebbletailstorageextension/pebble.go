// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"bytes"
	"os"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/bloom"
	"go.uber.org/zap"
)

func newPebbleDB(storageDir string, logger *zap.Logger) (db *pebble.DB, created bool, err error) {
	cache := pebble.NewCache(16 << 20)
	defer cache.Unref()

	opts := &pebble.Options{
		FormatMajorVersion: pebble.FormatValueSeparation,
		Logger:             logger.Sugar(),
		MemTableSize:       32 << 20,
		Comparer:           traceKeyComparer(),
		Cache:              cache,
		CompactionConcurrencyRange: func() (lower, upper int) {
			return 2, 4
		},
	}
	opts.WithFSDefaults()
	opts.Experimental.L0CompactionConcurrency = 4

	opts.Levels[0] = pebble.LevelOptions{
		BlockSize:    32 << 10,
		FilterPolicy: bloom.FilterPolicy(10),
		FilterType:   pebble.TableFilter,
	}

	desc, err := pebble.Peek(storageDir, opts.FS)
	switch {
	case err != nil && os.IsNotExist(err):
		created = true
	case err != nil:
		return nil, false, err
	default:
		created = !desc.Exists
	}

	db, err = pebble.Open(storageDir, opts)
	return db, created, err
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
