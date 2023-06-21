// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type EmitFunc func(ctx context.Context, attrs *FileAttributes, token []byte)

type Manager struct {
	*zap.SugaredLogger
	wg     sync.WaitGroup
	cancel context.CancelFunc

	readerFactory readerFactory
	finder        Finder
	roller        roller
	persister     operator.Persister

	pollInterval    time.Duration
	maxBatches      int
	maxBatchFiles   int
	deleteAfterRead bool

	knownFiles []*Reader
	seenPaths  map[string]struct{}

	currentFps []*Fingerprint
}

func (m *Manager) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.persister = persister

	// Load offsets from disk
	if err := m.loadLastPollFiles(ctx); err != nil {
		return fmt.Errorf("read known files from database: %w", err)
	}

	if files, err := m.finder.FindFiles(); err != nil {
		return fmt.Errorf("find files: %w", err)
	} else if len(files) == 0 {
		m.Warnw("no files match the configured include patterns",
			"include", m.finder.Include,
			"exclude", m.finder.Exclude)
	}

	// Start polling goroutine
	m.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (m *Manager) Stop() error {
	m.cancel()
	m.wg.Wait()
	m.roller.cleanup()
	for _, reader := range m.knownFiles {
		reader.Close()
	}
	m.knownFiles = nil
	m.cancel = nil
	return nil
}

// startPoller kicks off a goroutine that will poll the filesystem periodically,
// checking if there are new files or new logs in the watched files
func (m *Manager) startPoller(ctx context.Context) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		globTicker := time.NewTicker(m.pollInterval)
		defer globTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-globTicker.C:
			}

			m.poll(ctx)
		}
	}()
}

// poll checks all the watched paths for new entries
func (m *Manager) poll(ctx context.Context) {
	// Increment the generation on all known readers
	// This is done here because the next generation is about to start
	for i := 0; i < len(m.knownFiles); i++ {
		m.knownFiles[i].generation++
	}

	// Used to keep track of the number of batches processed in this poll cycle
	batchesProcessed := 0

	// Get the list of paths on disk
	matches, err := m.finder.FindFiles()
	if err != nil {
		m.Errorf("error finding files: %s", err)
	}

	for len(matches) > m.maxBatchFiles {
		m.consume(ctx, matches[:m.maxBatchFiles])

		// If a maxBatches is set, check if we have hit the limit
		if m.maxBatches != 0 {
			batchesProcessed++
			if batchesProcessed >= m.maxBatches {
				return
			}
		}

		matches = matches[m.maxBatchFiles:]
	}
	m.consume(ctx, matches)
}

func (m *Manager) consume(ctx context.Context, paths []string) {
	m.Debug("Consuming files")
	readers := make([]*Reader, 0, len(paths))
	for _, path := range paths {
		r := m.makeReader(path)
		if r != nil {
			readers = append(readers, r)
		}
	}

	// take care of files which disappeared from the pattern since the last poll cycle
	// this can mean either files which were removed, or rotated into a name not matching the pattern
	// we do this before reading existing files to ensure we emit older log lines before newer ones
	m.roller.readLostFiles(ctx, readers)

	var wg sync.WaitGroup
	for _, reader := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
			// Delete a file if deleteAfterRead is enabled and we reached the end of the file
			if m.deleteAfterRead && r.eof {
				r.Close()
				if err := os.Remove(r.file.Name()); err != nil {
					m.Errorf("could not delete %s", r.file.Name())
				}
			}
		}(reader)
	}
	wg.Wait()

	// Save off any files that were not fully read
	if m.deleteAfterRead {
		unfinished := make([]*Reader, 0, len(readers))
		for _, r := range readers {
			if !r.eof {
				unfinished = append(unfinished, r)
			}
		}
		readers = unfinished

		// If all files were read and deleted then no need to do bookkeeping on readers
		if len(readers) == 0 {
			return
		}
	}

	// Any new files that appear should be consumed entirely
	m.readerFactory.fromBeginning = true

	m.roller.roll(ctx, readers)
	m.saveCurrent(readers)
	m.syncLastPollFiles(ctx)
	m.clearCurrentFingerprints()
}

func (m *Manager) makeFingerprint(path string) (*Fingerprint, *os.File) {
	if _, ok := m.seenPaths[path]; !ok {
		if m.readerFactory.fromBeginning {
			m.Infow("Started watching file", "path", path)
		} else {
			m.Infow("Started watching file from end. To read preexisting logs, configure the argument 'start_at' to 'beginning'", "path", path)
		}
		m.seenPaths[path] = struct{}{}
	}
	file, err := os.Open(path) // #nosec - operator must read in files defined by user
	if err != nil {
		m.Debugf("Failed to open file", zap.Error(err))
		return nil, nil
	}

	fp, err := m.readerFactory.newFingerprint(file)
	if err != nil {
		if err = file.Close(); err != nil {
			m.Errorf("problem closing file %s", file.Name())
		}
		return nil, nil
	}

	if len(fp.FirstBytes) == 0 {
		// Empty file, don't read it until we can compare its fingerprint
		if err = file.Close(); err != nil {
			m.Errorf("problem closing file %s", file.Name())
		}
		return nil, nil
	}
	return fp, file
}

func (m *Manager) checkDuplicates(fp *Fingerprint) bool {
	for i := 0; i < len(m.currentFps); i++ {
		fp2 := m.currentFps[i]
		if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
			return true
		}
	}
	return false
}

// makeReader take a file path, then creates reader,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (m *Manager) makeReader(path string) *Reader {
	// Open the files first to minimize the time between listing and opening
	fp, file := m.makeFingerprint(path)
	if fp == nil {
		return nil
	}

	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files
	if m.checkDuplicates(fp) {
		if err := file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		return nil
	}

	m.currentFps = append(m.currentFps, fp)
	reader, err := m.newReader(file, fp)
	if err != nil {
		m.Errorw("Failed to create reader", zap.Error(err))
		return nil
	}

	return reader
}

func (m *Manager) clearCurrentFingerprints() {
	m.currentFps = make([]*Fingerprint, 0)
}

// saveCurrent adds the readers from this polling interval to this list of
// known files, then increments the generation of all tracked old readers
// before clearing out readers that have existed for 3 generations.
func (m *Manager) saveCurrent(readers []*Reader) {
	// Add readers from the current, completed poll interval to the list of known files
	m.knownFiles = append(m.knownFiles, readers...)

	// Clear out old readers. They are sorted such that they are oldest first,
	// so we can just find the first reader whose generation is less than our
	// max, and keep every reader after that
	for i := 0; i < len(m.knownFiles); i++ {
		reader := m.knownFiles[i]
		if reader.generation <= 3 {
			m.knownFiles = m.knownFiles[i:]
			break
		}
	}
}

func (m *Manager) newReader(file *os.File, fp *Fingerprint) (*Reader, error) {
	// Check if the new path has the same fingerprint as an old path
	if oldReader, ok := m.findFingerprintMatch(fp); ok {
		return m.readerFactory.copy(oldReader, file)
	}

	// If we don't match any previously known files, create a new reader from scratch
	return m.readerFactory.newReader(file, fp)
}

func (m *Manager) findFingerprintMatch(fp *Fingerprint) (*Reader, bool) {
	// Iterate backwards to match newest first
	for i := len(m.knownFiles) - 1; i >= 0; i-- {
		oldReader := m.knownFiles[i]
		if fp.StartsWith(oldReader.Fingerprint) {
			// Remove the old reader from the list of known files. We will
			// add it back in saveCurrent if it is still alive.
			m.knownFiles = append(m.knownFiles[:i], m.knownFiles[i+1:]...)
			return oldReader, true
		}
	}
	return nil, false
}

const knownFilesKey = "knownFiles"

// syncLastPollFiles syncs the most recent set of files to the database
func (m *Manager) syncLastPollFiles(ctx context.Context) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Encode the number of known files
	if err := enc.Encode(len(m.knownFiles)); err != nil {
		m.Errorw("Failed to encode known files", zap.Error(err))
		return
	}

	// Encode each known file
	for _, fileReader := range m.knownFiles {
		if err := enc.Encode(fileReader); err != nil {
			m.Errorw("Failed to encode known files", zap.Error(err))
		}
	}

	if err := m.persister.Set(ctx, knownFilesKey, buf.Bytes()); err != nil {
		m.Errorw("Failed to sync to database", zap.Error(err))
	}
}

// syncLastPollFiles loads the most recent set of files to the database
func (m *Manager) loadLastPollFiles(ctx context.Context) error {
	encoded, err := m.persister.Get(ctx, knownFilesKey)
	if err != nil {
		return err
	}

	if encoded == nil {
		m.knownFiles = make([]*Reader, 0, 10)
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(encoded))

	// Decode the number of entries
	var knownFileCount int
	if err := dec.Decode(&knownFileCount); err != nil {
		return fmt.Errorf("decoding file count: %w", err)
	}

	if knownFileCount > 0 {
		m.Infow("Resuming from previously known offset(s). 'start_at' setting is not applicable.")
		m.readerFactory.fromBeginning = true
	}

	// Decode each of the known files
	m.knownFiles = make([]*Reader, 0, knownFileCount)
	for i := 0; i < knownFileCount; i++ {
		// Only the offset, fingerprint, and splitter
		// will be used before this reader is discarded
		unsafeReader, err := m.readerFactory.unsafeReader()
		if err != nil {
			return err
		}
		if err = dec.Decode(unsafeReader); err != nil {
			return err
		}
		m.knownFiles = append(m.knownFiles, unsafeReader)
	}

	return nil
}
