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
	return &detectLostFiles{[]*reader{}}
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
