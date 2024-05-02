// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type Manager struct {
	*zap.SugaredLogger
	wg     sync.WaitGroup
	cancel context.CancelFunc

	readerFactory reader.Factory
	fileMatcher   *matcher.Matcher
	tracker       tracker.Tracker

	pollInterval  time.Duration
	persister     operator.Persister
	maxBatches    int
	maxBatchFiles int
}

func (m *Manager) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	if _, err := m.fileMatcher.MatchFiles(); err != nil {
		m.Warnf("finding files: %v", err)
	}

	if persister != nil {
		m.persister = persister
		offsets, err := checkpoint.Load(ctx, m.persister)
		if err != nil {
			return fmt.Errorf("read known files from database: %w", err)
		}
		if len(offsets) > 0 {
			m.Infow("Resuming from previously known offset(s). 'start_at' setting is not applicable.")
			m.readerFactory.FromBeginning = true
			m.tracker.LoadMetadata(offsets)
		}
	}

	// Start polling goroutine
	m.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (m *Manager) Stop() error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.wg.Wait()
	m.tracker.ClosePreviousFiles()
	if m.persister != nil {
		if err := checkpoint.Save(context.Background(), m.persister, m.tracker.GetMetadata()); err != nil {
			m.Errorw("save offsets", zap.Error(err))
		}
	}
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
	// Used to keep track of the number of batches processed in this poll cycle
	batchesProcessed := 0

	// Get the list of paths on disk
	matches, err := m.fileMatcher.MatchFiles()
	if err != nil {
		m.Debugf("finding files: %v", err)
	}
	m.Debugw("matched files", zap.Strings("paths", matches))

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

	// Any new files that appear should be consumed entirely
	m.readerFactory.FromBeginning = true
	if m.persister != nil {
		metadata := m.tracker.GetMetadata()
		if metadata != nil {
			if err := checkpoint.Save(context.Background(), m.persister, metadata); err != nil {
				m.Errorw("save offsets", zap.Error(err))
			}
		}
	}
	// rotate at end of every poll()
	m.tracker.EndPoll()
}

func (m *Manager) consume(ctx context.Context, paths []string) {
	m.Debug("Consuming files", zap.Strings("paths", paths))
	m.makeReaders(paths)

	m.readLostFiles(ctx)

	// read new readers to end
	var wg sync.WaitGroup
	for _, r := range m.tracker.CurrentPollFiles() {
		wg.Add(1)
		go func(r *reader.Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
		}(r)
	}
	wg.Wait()

	m.tracker.EndConsume()
}

func (m *Manager) makeFingerprint(path string) (*fingerprint.Fingerprint, *os.File) {
	file, err := os.Open(path) // #nosec - operator must read in files defined by user
	if err != nil {
		m.Errorw("Failed to open file", zap.Error(err))
		return nil, nil
	}

	fp, err := m.readerFactory.NewFingerprint(file)
	if err != nil {
		if err = file.Close(); err != nil {
			m.Debugw("problem closing file", zap.Error(err))
		}
		return nil, nil
	}

	if fp.Len() == 0 {
		// Empty file, don't read it until we can compare its fingerprint
		if err = file.Close(); err != nil {
			m.Debugw("problem closing file", zap.Error(err))
		}
		return nil, nil
	}
	return fp, file
}

// makeReader take a file path, then creates reader,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (m *Manager) makeReaders(paths []string) {
	for _, path := range paths {
		fp, file := m.makeFingerprint(path)
		if fp == nil {
			continue
		}

		// Exclude duplicate paths with the same content. This can happen when files are
		// being rotated with copy/truncate strategy. (After copy, prior to truncate.)
		if r := m.tracker.GetCurrentFile(fp); r != nil {
			// re-add the reader as Match() removes duplicates
			m.tracker.Add(r)
			if err := file.Close(); err != nil {
				m.Debugw("problem closing file", zap.Error(err))
			}
			continue
		}

		r, err := m.newReader(file, fp)
		if err != nil {
			m.Errorw("Failed to create reader", zap.Error(err))
			continue
		}

		m.tracker.Add(r)
	}
}

func (m *Manager) newReader(file *os.File, fp *fingerprint.Fingerprint) (*reader.Reader, error) {
	// Check previous poll cycle for match
	if oldReader := m.tracker.GetOpenFile(fp); oldReader != nil {
		return m.readerFactory.NewReaderFromMetadata(file, oldReader.Close())
	}

	// Check for closed files for match
	if oldMetadata := m.tracker.GetClosedFile(fp); oldMetadata != nil {
		return m.readerFactory.NewReaderFromMetadata(file, oldMetadata)
	}

	// If we don't match any previously known files, create a new reader from scratch
	m.Infow("Started watching file", "path", file.Name())
	return m.readerFactory.NewReader(file, fp)
}
