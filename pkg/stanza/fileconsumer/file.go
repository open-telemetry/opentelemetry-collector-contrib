// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	pollerWg sync.WaitGroup
	workerWg sync.WaitGroup
	cancel   context.CancelFunc
	ctx      context.Context

	readerFactory readerFactory
	finder        Finder
	roller        roller
	persister     operator.Persister

	pollInterval    time.Duration
	maxBatchFiles   int
	deleteAfterRead bool

	knownFiles     []*Reader
	knownFilesLock sync.RWMutex
	seenPaths      map[string]struct{}

	currentFps []*Fingerprint

	readerChan chan ReaderWrapper
	trieLock   sync.RWMutex

	// readers[] store the readers of previous poll cycles and are used to keep track of lost files
	readerLock sync.Mutex
	readers    []*Reader

	// TRIE - this data structure stores the fingerprint of the files which are currently being consumed
	trie *Trie
}

func (m *Manager) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.ctx = ctx
	m.persister = persister
	m.readerChan = make(chan ReaderWrapper, m.maxBatchFiles)

	// Load offsets from disk
	if err := m.loadLastPollFiles(ctx); err != nil {
		return fmt.Errorf("read known files from database: %w", err)
	}

	if len(m.finder.FindFiles()) == 0 {
		m.Warnw("no files match the configured include patterns",
			"include", m.finder.Include,
			"exclude", m.finder.Exclude)
	}

	for i := 0; i < m.maxBatchFiles; i++ {
		m.workerWg.Add(1)
		go m.worker(ctx)
	}

	// Start polling goroutine
	m.startPoller(ctx)

	return nil
}

// Stop will stop the file monitoring process
func (m *Manager) Stop() error {
	m.cancel()
	m.pollerWg.Wait()
	close(m.readerChan)
	m.workerWg.Wait()
	m.roller.cleanup()
	m.syncLastPollFiles(m.ctx)
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
	m.pollerWg.Add(1)
	go func() {
		defer m.pollerWg.Done()
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
	m.knownFilesLock.Lock()
	for i := 0; i < len(m.knownFiles); i++ {
		m.knownFiles[i].generation++
	}
	m.knownFilesLock.Unlock()

	// Get the list of paths on disk
	matches := m.finder.FindFiles()
	m.consume(ctx, matches)
	m.clearCurrentFingerprints()

	// Any new files that appear should be consumed entirely
	m.readerFactory.fromBeginning = true
	m.syncLastPollFiles(ctx)
}

func (m *Manager) clearCurrentFingerprints() {
	m.currentFps = make([]*Fingerprint, 0)
}

func (m *Manager) worker(ctx context.Context) {
	defer m.workerWg.Done()
	for {
		chanData, ok := <-m.readerChan

		if !ok {
			return
		}
		r, fp := chanData.reader, chanData.fp
		r.ReadToEnd(ctx)
		// Delete a file if deleteAfterRead is enabled and we reached the end of the file
		if m.deleteAfterRead && r.eof {
			r.Close()
			if err := os.Remove(r.file.Name()); err != nil {
				m.Errorf("could not delete %s", r.file.Name())
			}
		} else {
			// Save off any files that were not fully read or if deleteAfterRead is false
			m.saveCurrent([]*Reader{r})
			m.readerLock.Lock()
			m.readers = append(m.readers, r)
			m.readerLock.Unlock()
		}
		m.removePath(fp)
	}
}

func (m *Manager) consume(ctx context.Context, paths []string) {
	m.Debug("Consuming files")
	m.handleLostFiles(ctx)
	for _, path := range paths {
		reader, fp := m.makeReader(path)
		if reader == nil {
			fmt.Println("Couldn't create reader for ", path)
			continue
		}
		// add path and fingerprint as it's not consuming
		m.trieLock.Lock()
		m.trie.Put(fp.FirstBytes, true)
		m.trieLock.Unlock()
		m.readerChan <- ReaderWrapper{reader: reader, fp: fp}
	}
}

func (m *Manager) isCurrentlyConsuming(fp *Fingerprint) bool {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	return m.trie.Get(fp.FirstBytes) != nil
}

func (m *Manager) removePath(fp *Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Delete(fp.FirstBytes)
}

func (m *Manager) handleLostFiles(ctx context.Context) {
	m.readerLock.Lock()
	defer m.readerLock.Unlock()

	if m.deleteAfterRead {
		unfinished := make([]*Reader, 0, len(m.readers))
		for _, r := range m.readers {
			if !r.eof {
				unfinished = append(unfinished, r)
			}
		}
		m.readers = unfinished
		if len(m.readers) == 0 {
			return
		}
	}
	m.rollReaders(ctx, m.readers)
	m.readers = make([]*Reader, 0)
}

func (m *Manager) rollReaders(ctx context.Context, readers []*Reader) {
	m.roller.readLostFiles(ctx, readers)
	m.roller.roll(ctx, readers)
	m.clearOldReaders()
}

func (m *Manager) makeReader(filePath string) (*Reader, *Fingerprint) {
	if _, ok := m.seenPaths[filePath]; !ok {
		if m.readerFactory.fromBeginning {
			m.Infow("Started watching file", "path", filePath)
		} else {
			m.Infow("Started watching file from end. To read preexisting logs, configure the argument 'start_at' to 'beginning'", "path", filePath)
		}
		m.seenPaths[filePath] = struct{}{}
	}
	file, err := os.Open(filePath) // #nosec - operator must read in files defined by user
	if err != nil {
		m.Debugf("Failed to open file", zap.Error(err))
		return nil, nil
	}
	fp, err := m.readerFactory.newFingerprint(file)
	if err != nil {
		m.Errorw("Failed creating fingerprint", zap.Error(err))
		return nil, nil
	}
	// Exclude any empty fingerprints or duplicate fingerprints to avoid doubling up on copy-truncate files

	if len(fp.FirstBytes) == 0 {
		if err = file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		return nil, nil
	}

	// check if the current file is already being consumed
	if m.isCurrentlyConsuming(fp) {
		if err = file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		return nil, nil
	}
	m.currentFps = append(m.currentFps, fp)

	for i := 0; i < len(m.currentFps)-1; i++ {
		fp2 := m.currentFps[i]
		if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
			// Exclude
			if err = file.Close(); err != nil {
				m.Errorf("problem closing file", "file", file.Name())
			}
			m.currentFps = m.currentFps[:len(m.currentFps)-1]
			return nil, nil
		}
	}

	reader, err := m.newReader(file, fp)
	if err != nil {
		m.Errorw("Failed to create reader", zap.Error(err))
		return nil, nil
	}
	return reader, fp
}

// saveCurrent adds the readers from this polling interval to this list of
// known files, then increments the generation of all tracked old readers
// before clearing out readers that have existed for 3 generations.
func (m *Manager) saveCurrent(readers []*Reader) {
	// Add readers from the current, completed poll interval to the list of known files
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	m.knownFiles = append(m.knownFiles, readers...)
}

func (m *Manager) clearOldReaders() {
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
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
	return m.readerFactory.newReader(file, fp.Copy())
}

func (m *Manager) findFingerprintMatch(fp *Fingerprint) (*Reader, bool) {
	// Iterate backwards to match newest first
	m.knownFilesLock.RLock()
	defer m.knownFilesLock.RUnlock()
	for i := len(m.knownFiles) - 1; i >= 0; i-- {
		oldReader := m.knownFiles[i]
		if fp.StartsWith(oldReader.Fingerprint) {
			return oldReader, true
		}
	}
	return nil, false
}

const knownFilesKey = "knownFiles"

// syncLastPollFiles syncs the most recent set of files to the database
func (m *Manager) syncLastPollFiles(ctx context.Context) {
	m.knownFilesLock.RLock()
	defer m.knownFilesLock.RUnlock()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// No need to encode if knownFiles is empty
	if len(m.knownFiles) == 0 {
		return
	}

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
