// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package tracker

import (
	"context"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

func (t *Tracker) PreConsume(ctx context.Context) {
	previousPollFiles := t.openFiles.readers
	lostReaders := make([]*reader.Reader, 0, len(previousPollFiles))
OUTER:
	for _, oldReader := range previousPollFiles {
		for _, newReader := range t.ActiveFiles() {
			if newReader.Fingerprint.StartsWith(oldReader.Fingerprint) {
				continue OUTER
			}

			if !newReader.NameEquals(oldReader) {
				continue
			}

			// At this point, we know that the file has been rotated. However, we do not know
			// if it was moved or truncated. If truncated, then both handles point to the same
			// file, in which case we should only read from it using the new reader. We can use
			// the Validate method to ensure that the file has not been truncated.
			if !oldReader.Validate() {
				continue OUTER
			}
		}
		lostReaders = append(lostReaders, oldReader)
	}

	var lostWG sync.WaitGroup
	for _, lostReader := range lostReaders {
		lostWG.Add(1)
		go func(r *reader.Reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
		}(lostReader)
	}
	lostWG.Wait()
}

// On non-windows platforms, we keep files open between poll cycles so that we can detect
// and read "lost" files, which have been moved out of the matching pattern.
func (t *Tracker) PostConsume() {
	// close open files and move them to closed fileset
	// move active fileset to open fileset
	// empty out active fileset
	t.closePreviousFiles()
	t.openFiles.readers = t.activeFiles.readers
	t.activeFiles.Clear()
}
