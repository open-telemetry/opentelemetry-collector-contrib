// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"sync"
)

type detectLostFiles struct {
	oldReaders []*reader
}

func newRoller() roller {
	return &detectLostFiles{oldReaders: []*reader{}}
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
				// file, in which case we should only read from it using the new reader. We can use
				// the validateFingerprint method to establish that the file has not been truncated.
				if !oldReader.validateFingerprint() {
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
