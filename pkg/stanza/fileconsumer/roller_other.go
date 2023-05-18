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
	oldReaders []*Reader
}

func newRoller() roller {
	return &detectLostFiles{[]*Reader{}}
}

func (r *detectLostFiles) readLostFiles(ctx context.Context, readers []*Reader) {
	// Detect files that have been rotated out of matching pattern
	lostReaders := make([]*Reader, 0, len(r.oldReaders))
OUTER:
	for _, oldReader := range r.oldReaders {
		for _, reader := range readers {
			if reader.Fingerprint.StartsWith(oldReader.Fingerprint) {
				continue OUTER
			}
		}
		lostReaders = append(lostReaders, oldReader)
	}

	var lostWG sync.WaitGroup
	for _, reader := range lostReaders {
		lostWG.Add(1)
		go func(r *Reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
		}(reader)
	}
	lostWG.Wait()
}

func (r *detectLostFiles) roll(ctx context.Context, readers []*Reader) {
	for _, reader := range r.oldReaders {
		reader.Close()
	}

	r.oldReaders = readers
}

func (r *detectLostFiles) cleanup() {
	for _, reader := range r.oldReaders {
		reader.Close()
	}
}
