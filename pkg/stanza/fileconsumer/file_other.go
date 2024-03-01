// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

// Take care of files which disappeared from the pattern since the last poll cycle
// this can mean either files which were removed, or rotated into a name not matching the pattern
// we do this before reading existing files to ensure we emit older log lines before newer ones
func (m *Manager) readLostFiles(ctx context.Context) {
	if m.readerFactory.DeleteAtEOF {
		// Lost files are not expected when delete_at_eof is enabled
		// since we are deleting the files before they can become lost.
		return
	}
	lostReaders := make([]*reader.Reader, 0, m.previousPollFiles.Len())
OUTER:
	for _, oldReader := range m.previousPollFiles.Get() {
		for _, newReader := range m.currentPollFiles.Get() {
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
func (m *Manager) postConsume() {
	m.closePreviousFiles()

	// m.currentPollFiles -> m.previousPollFiles
	m.previousPollFiles = m.currentPollFiles
}
