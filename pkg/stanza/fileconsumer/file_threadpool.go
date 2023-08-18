// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
)

type readerWrapper struct {
	reader *reader
	fp     *fingerprint.Fingerprint
}

func (m *Manager) kickoffThreads(ctx context.Context) {
	m.readerChan = make(chan readerWrapper, m.maxBatchFiles*2)
	for i := 0; i < m.maxBatchFiles; i++ {
		m.workerWg.Add(1)
		go m.worker(ctx)
	}
}

func (m *Manager) shutdownThreads() {
	if m.readerChan != nil {
		close(m.readerChan)
	}
	m.workerWg.Wait()
	// save off any files left
	// As we already cancelled our current context, create a new one to save any left offsets
	// This is only applicable for `filelog.useThreadPool` featuregate
	ctx, cancel := context.WithCancel(context.Background())
	m.syncLastPollFilesConcurrent(ctx)
	cancel()
}

// poll checks all the watched paths for new entries
func (m *Manager) pollConcurrent(ctx context.Context) {
	// Increment the generation on all known readers
	// This is done here because the next generation is about to start
	m.knownFilesLock.Lock()
	for i := 0; i < len(m.knownFiles); i++ {
		m.knownFiles[i].generation++
	}
	m.knownFilesLock.Unlock()

	// Get the list of paths on disk
	matches, err := m.fileMatcher.MatchFiles()
	if err != nil {
		m.Errorf("error finding files: %s", err)
	}
	m.consumeConcurrent(ctx, matches)
	m.clearCurrentFingerprints()

	// Any new files that appear should be consumed entirely
	m.readerFactory.fromBeginning = true
	m.syncLastPollFilesConcurrent(ctx)
}

func (m *Manager) worker(ctx context.Context) {
	defer m.workerWg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case chanData, ok := <-m.readerChan:
			if !ok {
				return
			}
			r, fp := chanData.reader, chanData.fp
			if !m.readToEnd(ctx, r) {
				// Save off any files that were not fully read or if deleteAfterRead is disabled
				m.knownFilesLock.Lock()
				m.knownFiles = append(m.knownFiles, r)
				m.knownFilesLock.Unlock()
			}
			m.removePath(fp)
		}

	}
}

func (m *Manager) makeReaderConcurrent(filePath string) (*reader, *fingerprint.Fingerprint) {
	fp, file := m.makeFingerprint(filePath)
	if fp == nil {
		return nil, nil
	}

	// check if the current file is already being consumed
	if m.isCurrentlyConsuming(fp) {
		if err := file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		return nil, nil
	}

	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files
	if m.checkDuplicates(fp) {
		if err := file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		return nil, nil
	}
	m.currentFps = append(m.currentFps, fp)

	reader, err := m.newReaderConcurrent(file, fp)
	if err != nil {
		m.Errorw("Failed to create reader", zap.Error(err))
		return nil, nil
	}
	return reader, fp
}

func (m *Manager) consumeConcurrent(ctx context.Context, paths []string) {
	m.clearOldReadersConcurrent(ctx)
	for _, path := range paths {
		reader, fp := m.makeReaderConcurrent(path)
		if reader != nil {
			// add path and fingerprint as it's not consuming
			m.trieLock.Lock()
			m.trie.Put(fp.FirstBytes)
			m.trieLock.Unlock()
			m.readerChan <- readerWrapper{reader: reader, fp: fp}
		}
	}
}

func (m *Manager) isCurrentlyConsuming(fp *fingerprint.Fingerprint) bool {
	m.trieLock.RLock()
	defer m.trieLock.RUnlock()
	return m.trie.HasKey(fp.FirstBytes)
}

func (m *Manager) removePath(fp *fingerprint.Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Delete(fp.FirstBytes)
}

func (m *Manager) clearOldReadersConcurrent(ctx context.Context) {
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	// Clear out old readers. They are sorted such that they are oldest first,
	// so we can just find the first reader whose poll cycle is less than our
	// limit i.e. last 3 cycles, and keep every reader after that
	oldReaders := make([]*reader, 0)
	i := 0
	for ; i < len(m.knownFiles); i++ {
		reader := m.knownFiles[i]
		if reader.generation >= 3 {
			oldReaders = append(oldReaders, reader)
		} else {
			break
		}
	}
	m.knownFiles = m.knownFiles[i:]

	var lostWG sync.WaitGroup
	for _, r := range oldReaders {
		lostWG.Add(1)
		go func(r *reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
			r.Close()
		}(r)
	}
	lostWG.Wait()
}

func (m *Manager) newReaderConcurrent(file *os.File, fp *fingerprint.Fingerprint) (*reader, error) {
	// Check if the new path has the same fingerprint as an old path
	if oldReader, ok := m.findFingerprintMatchConcurrent(fp); ok {
		return m.readerFactory.copy(oldReader, file)
	}

	// If we don't match any previously known files, create a new reader from scratch
	return m.readerFactory.newReader(file, fp.Copy())
}

func (m *Manager) findFingerprintMatchConcurrent(fp *fingerprint.Fingerprint) (*reader, bool) {
	// Iterate backwards to match newest first
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()

	return m.findFingerprintMatch(fp)
}

// syncLastPollFiles syncs the most recent set of files to the database
func (m *Manager) syncLastPollFilesConcurrent(ctx context.Context) {
	m.knownFilesLock.RLock()
	defer m.knownFilesLock.RUnlock()

	m.syncLastPollFiles(ctx)
}
