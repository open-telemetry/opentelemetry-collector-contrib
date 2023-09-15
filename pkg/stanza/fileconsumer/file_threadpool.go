// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"io"
	"os"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
)

type readerEnvelope struct {
	reader  *reader
	trieKey *fingerprint.Fingerprint
}

func (m *Manager) kickoffThreads(ctx context.Context) {
	m.readerChan = make(chan readerEnvelope, m.maxBatchFiles)
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
	batchesProcessed := 0
	// Get the list of paths on disk
	matches, err := m.fileMatcher.MatchFiles()
	if err != nil {
		m.Errorf("error finding files: %s", err)
	}
	for len(matches) > m.maxBatchFiles {
		m.consumeConcurrent(ctx, matches[:m.maxBatchFiles])

		if m.maxBatches != 0 {
			batchesProcessed++
			if batchesProcessed >= m.maxBatches {
				return
			}
		}

		matches = matches[m.maxBatchFiles:]
	}
	m.consumeConcurrent(ctx, matches)

	// Any new files that appear should be consumed entirely
	m.readerFactory.fromBeginning = true
	m.syncLastPollFilesConcurrent(ctx)
}

func (m *Manager) worker(ctx context.Context) {
	defer m.workerWg.Done()
	for {
		select {
		case wrapper, ok := <-m.readerChan:
			if !ok {
				return
			}
			r, fp := wrapper.reader, wrapper.trieKey
			r.ReadToEnd(ctx)
			if m.deleteAfterRead && r.eof {
				r.Delete()
			} else {
				// Save off any files that were not fully read.
				m.knownFilesLock.Lock()
				m.knownFiles = append(m.knownFiles, r)
				m.knownFilesLock.Unlock()
			}
			m.trieDelete(fp)
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
	m.Debug("Consuming files")
	m.clearOldReadersConcurrent(ctx)
	for _, path := range paths {
		reader, fp := m.makeReaderConcurrent(path)
		if reader != nil {
			// add fingerprint to trie
			m.triePut(fp)
			m.readerChan <- readerEnvelope{reader: reader, trieKey: fp}
		}
	}
	m.clearCurrentFingerprints()
}

func (m *Manager) isCurrentlyConsuming(fp *fingerprint.Fingerprint) bool {
	m.trieLock.RLock()
	defer m.trieLock.RUnlock()
	return m.trie.HasKey(fp.FirstBytes)
}

func (m *Manager) triePut(fp *fingerprint.Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Put(fp.FirstBytes)
}

func (m *Manager) trieDelete(fp *fingerprint.Fingerprint) {
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
	i := 0
	for ; i < len(m.knownFiles); i++ {
		reader := m.knownFiles[i]
		if reader.generation <= 3 {
			break
		}
	}
	m.knownFiles = m.knownFiles[i:]
	var wg sync.WaitGroup
	for _, r := range m.knownFiles {
		if r.file == nil {
			// already closed
			continue
		}
		if m.checkTruncate(r) {
			// if it's an updated version, we don't to readToEnd, it will cause duplicates.
			// current poll() will take care of reading
			r.Close()
		} else {
			wg.Add(1)
			go func(r *reader) {
				defer wg.Done()
				r.ReadToEnd(ctx)
				r.Close()
			}(r)
		}
	}
	wg.Wait()
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

// check if current file is a different file after copy-truncate
func (m *Manager) checkTruncate(r *reader) bool {
	/*
		Suppose file.log with content "ABCDEG" gets truncated to "".
		But it has the cursor at position "5" (i.e. 'G') in memory.

		If the updated file.log has it's content "QWERTYXYZ",
		next call to read() on previously opened file with return "XYZ".
		This is undesriable and will cause duplication.

		NOTE: we haven't closed the previouly opened file
		Check if it's updated version of previously opened file.
	*/

	// store current offset
	oldOffset := r.Offset
	oldCursor, err := r.file.Seek(0, 1)
	if err != nil {
		m.Errorw("Failed to seek", err)
		return false
	}

	r.file.Seek(0, 0)
	new := make([]byte, r.fingerprintSize)
	n, err := r.file.Read(new)
	if err != nil && err != io.EOF {
		m.Errorw("Failed to read", err)
		return false
	}

	// restore the offset in case if it's not a truncate
	r.Offset = oldOffset
	_, err = r.file.Seek(oldCursor, 0)
	if err != nil {
		m.Errorw("Failed to seek", err)
		return false
	}

	newFp := fingerprint.Fingerprint{FirstBytes: new[:n]}
	return !newFp.StartsWith(r.Fingerprint)
}
