// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
)

type detectLostFiles struct {
	oldReaders      []*reader
	fingerprintSize int // used when we check for truncation
}

func newRoller(fingerprintSize int) roller {
	return &detectLostFiles{
		oldReaders:      []*reader{},
		fingerprintSize: fingerprintSize,
	}
}

func (r *detectLostFiles) readLostFiles(ctx context.Context, newReaders []*reader) {
	// Detect files that have been rotated out of matching pattern
	lostReaders := make([]*reader, 0, len(r.oldReaders))
OUTER:
	for _, oldReader := range r.oldReaders {
		for _, newReader := range newReaders {
			if newReader.Fingerprint.StartsWith(oldReader.Fingerprint) {
				continue OUTER
			}
			if oldReader.fileName == newReader.fileName {
				// At this point, we know that the file has been rotated. However, we do not know
				// if it was moved or truncated. If truncated, then both handles point to the same
				// file, in which case we should only read from it using the new reader.

				// We can detect truncation by recreating a fingerprint from the old handle.
				// If it matches the old fingerprint, then we know that the file was moved,
				// so we can consider the file lost and continue reading from the old handle.
				// If there's an error reading a new fingerprint from the old handle, let's assume we can't
				// read the rest of it anyways.
				refreshedFingerprint, err := fingerprint.New(oldReader.file, r.fingerprintSize)
				if err == nil && !refreshedFingerprint.StartsWith(oldReader.Fingerprint) {
					continue OUTER
				}
			}
		}
		lostReaders = append(lostReaders, oldReader)
	}

	var lostWG sync.WaitGroup
	for _, lostReader := range lostReaders {
		lostWG.Add(1)
		go func(r *reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
		}(lostReader)
	}
	lostWG.Wait()
}

func (r *detectLostFiles) roll(_ context.Context, newReaders []*reader) {
	for _, oldReader := range r.oldReaders {
		oldReader.Close()
	}

	r.oldReaders = newReaders
}

func (r *detectLostFiles) cleanup() {
	for _, oldReader := range r.oldReaders {
		oldReader.Close()
	}
}
