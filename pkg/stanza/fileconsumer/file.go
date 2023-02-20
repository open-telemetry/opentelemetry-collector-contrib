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
	wg     sync.WaitGroup
	_wg    sync.WaitGroup
	cancel context.CancelFunc

	readerFactory readerFactory
	finder        Finder
	roller        roller
	persister     operator.Persister

	pollInterval    time.Duration
	maxBatches      int
	maxBatchFiles   int
	deleteAfterRead bool

	knownFiles      []*Reader
	knownFilesLock  sync.RWMutex
	seenPaths       map[string]struct{}
	currentFiles    []*os.File
	currentFps      []*Fingerprint
	currentReaders  []*Reader
	readerChan      chan ReaderWrapper
	readerCloseChan chan *Reader
	queueHash       map[string]bool
	queueHashMtx    sync.RWMutex
	readerLock      sync.Mutex
}

func (m *Manager) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.persister = persister

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
		m._wg.Add(1)
		go m.worker(ctx)
	}
	go m.handleLostFiles(ctx)

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
	// batchesProcessed := 0

	// Get the list of paths on disk
	matches := m.finder.FindFiles()
	// for len(matches) > m.maxBatchFiles {
	// 	m.consume(ctx, matches[:m.maxBatchFiles])

	// 	// If a maxBatches is set, check if we have hit the limit
	// 	if m.maxBatches != 0 {
	// 		batchesProcessed++
	// 		if batchesProcessed >= m.maxBatches {
	// 			return
	// 		}
	// 	}

	// 	matches = matches[m.maxBatchFiles:]
	// }
	m.consume(ctx, matches)
	m.clearCurrentFiles()
}

func (m *Manager) clearCurrentFiles() {
	m.currentFiles = make([]*os.File, 0)
	m.currentFps = make([]*Fingerprint, 0)
}

func (m *Manager) worker(ctx context.Context) {
	defer m._wg.Done()
	for {
		chanData, ok := <-m.readerChan

		if !ok {
			break
		}
		r, path := chanData.reader, chanData.path
		r.ReadToEnd(ctx)
		if m.deleteAfterRead {
			r.Close()
			if err := os.Remove(r.file.Name()); err != nil {
				m.Errorf("could not delete %s", r.file.Name())
			}
		} else {
			m.readerCloseChan <- r
		}
		m.queueHashMtx.Lock()
		delete(m.queueHash, path)
		m.queueHashMtx.Unlock()
	}
}

func (m *Manager) consume(ctx context.Context, paths []string) {
	for _, path := range paths {
		m.queueHashMtx.Lock()
		if _, ok := m.queueHash[path]; ok {
			m.queueHashMtx.Unlock()
			continue
		}
		reader := m.makeReader(path)
		if reader == nil {
			fmt.Println("Couldn't create reader for ", path)
			m.queueHashMtx.Unlock()
			continue
		}
		m.queueHash[path] = true
		m.queueHashMtx.Unlock()
		m.readerChan <- ReaderWrapper{reader: reader, path: path}
	}
	if m.deleteAfterRead {
		return
	}
}

func (m *Manager) handleLostFiles(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	// ticker := time.NewTicker(10 * time.Millisecond)
	go func() {
		defer wg.Done()
		// readers := make([]*Reader, 0)
		if !m.deleteAfterRead {

			for {
				select {
				case reader := <-m.readerCloseChan:
					m.roller.readLostFiles(ctx, []*Reader{reader})
					m.roller.roll(ctx, []*Reader{reader})
					m.saveCurrent([]*Reader{reader})
					m.syncLastPollFiles(ctx)
					m.readerFactory.fromBeginning = true
					// case reader := <-m.readerCloseChan:
					// 	readers = append(readers, reader)
					// 	if len(readers)%m.maxBatchFiles == 0 {
					// 		m.roller.readLostFiles(ctx, readers)
					// 		m.roller.roll(ctx, readers)
					// 		m.saveCurrent(readers)
					// 		m.syncLastPollFiles(ctx)
					// 		readers = make([]*Reader, 0)
					// 	}
					// case <-ticker.C:
					// 	if len(readers) > 0 {
					// 		m.roller.readLostFiles(ctx, readers)
					// 		m.roller.roll(ctx, readers)
					// 		m.saveCurrent(readers)
					// 		m.syncLastPollFiles(ctx)
					// 		readers = make([]*Reader, 0)
					// 	}
					// }

				}
				// if len(readers) > 0 {
				// 	m.roller.readLostFiles(ctx, readers)
				// 	m.roller.roll(ctx, readers)
				// 	m.saveCurrent(readers)
				// 	m.syncLastPollFiles(ctx)
			}
		}
	}()
}

// makeReaders takes a list of paths, then creates readers from each of those paths,
// discarding any that have a duplicate fingerprint to other files that have already
// been read this polling interval
func (m *Manager) makeReaders(filePaths []string) []*Reader {
	readers := make([]*Reader, 0)
	for i := 0; i < len(filePaths); i++ {
		reader := m.makeReader(filePaths[i])
		if reader == nil {
			m.Errorw("Failed to create reader for path ")
			continue
		}
		readers = append(readers, reader)
	}
	return readers
}

func (m *Manager) makeReader(filePath string) *Reader {
	if _, ok := m.seenPaths[filePath]; !ok {
		if m.readerFactory.fromBeginning {
			m.Infow("Started watching file", "path", filePath)
		} else {
			m.Infow("Startedd watching file from end. To read preexisting logs, configure the argument 'start_at' to 'beginning'", "path", filePath)
		}
		m.seenPaths[filePath] = struct{}{}
	}
	file, err := os.Open(filePath) // #nosec - operator must read in files defined by user
	if err != nil {
		m.Debugf("Failed to open file", zap.Error(err))
		return nil
	}
	m.currentFiles = append(m.currentFiles, file)
	fp, err := m.readerFactory.newFingerprint(file)
	if err != nil {
		m.Errorw("Failed creating fingerprint", zap.Error(err))
		return nil
	}
	if len(fp.FirstBytes) == 0 {
		if err := file.Close(); err != nil {
			m.Errorf("problem closing file", "file", file.Name())
		}
		// Empty file, don't read it until we can compare its fingerprint
		m.currentFiles = m.currentFiles[:len(m.currentFiles)-1]
		return nil
	}
	m.currentFps = append(m.currentFps, fp)

	for i := 0; i < len(m.currentFps)-1; i++ {
		fp2 := m.currentFps[i]
		if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
			// Exclude
			if err := file.Close(); err != nil {
				m.Errorf("problem closing file", "file", file.Name())
			}
			m.currentFiles = m.currentFiles[:len(m.currentFiles)-1]
			m.currentFps = m.currentFps[:len(m.currentFps)-1]
			i--
			return nil
		}
	}

	reader, err := m.newReader(file, fp)
	if err != nil {
		m.Errorw("Failed to create reader", zap.Error(err))
		return nil
	}
	return reader
}

// saveCurrent adds the readers from this polling interval to this list of
// known files, then increments the generation of all tracked old readers
// before clearing out readers that have existed for 3 generations.
func (m *Manager) saveCurrent(readers []*Reader) {
	// Add readers from the current, completed poll interval to the list of known files
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	m.knownFiles = append(m.knownFiles, readers...)

	// Clear out old readers. They are sorted such that they are oldest first,
	// so we can just find the first reader whose generation is less than our
	// max, and keep every reader after that
	for i := 0; i < len(m.knownFiles); i++ {
		reader := m.knownFiles[i]
		if reader.generation <= 10 {
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
