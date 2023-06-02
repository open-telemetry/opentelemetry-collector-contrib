package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
)

type ReaderWrapper struct {
	reader *Reader
	fp     *Fingerprint
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
	matches := m.finder.FindFiles()
	m.consumeConcurrent(ctx, matches)
	m.clearCurrentFingerprints()

	// Any new files that appear should be consumed entirely
	m.readerFactory.fromBeginning = true
	m.syncLastPollFilesConcurrent(ctx)
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
			// Save off any files that were not fully read or if deleteAfterRead is disabled
			m.saveCurrentConcurrent(r)
		}
		m.removePath(fp)
	}
}

func (m *Manager) makeReaderConcurrent(filePath string) (*Reader, *Fingerprint) {
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

	for i := 0; i < len(m.currentFps)-1; i++ {
		fp2 := m.currentFps[i]
		if fp.StartsWith(fp2) || fp2.StartsWith(fp) {
			// Exclude
			if err = file.Close(); err != nil {
				m.Errorf("problem closing file", "file", file.Name())
			}
			return nil, nil
		}
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
			// add path and fingerprint as it's not consuming
			m.trieLock.Lock()
			m.trie.Put(fp.FirstBytes, true)
			m.trieLock.Unlock()
			m.readerChan <- ReaderWrapper{reader: reader, fp: fp}
		}
	}
}

func (m *Manager) isCurrentlyConsuming(fp *Fingerprint) bool {
	m.trieLock.RLock()
	defer m.trieLock.RUnlock()
	return m.trie.Get(fp.FirstBytes) != nil
}

func (m *Manager) removePath(fp *Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Delete(fp.FirstBytes)
}

// saveCurrent adds the readers from this polling interval to this list of
// known files, then increments the generation of all tracked old readers
// before clearing out readers that have existed for 3 generations.
func (m *Manager) saveCurrentConcurrent(reader *Reader) {
	// Add readers from the current, completed poll interval to the list of known files
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()

	m.knownFiles = append(m.knownFiles, reader)
}

func (m *Manager) clearOldReadersConcurrent(ctx context.Context) {
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	// Clear out old readers. They are sorted such that they are oldest first,
	// so we can just find the first reader whose poll cycle is less than our
	// limit i.e. last 3 cycles, and keep every reader after that
	oldReaders := make([]*Reader, 0)
	for i := 0; i < len(m.knownFiles); i++ {
		reader := m.knownFiles[i]
		if reader.generation <= 2 {
			oldReaders = m.knownFiles[:i]
			m.knownFiles = m.knownFiles[i:]
			break
		}
	}
	var lostWG sync.WaitGroup
	for _, reader := range oldReaders {
		lostWG.Add(1)
		go func(r *Reader) {
			defer lostWG.Done()
			r.ReadToEnd(ctx)
			r.Close()
		}(reader)
	}
	lostWG.Wait()
}

func (m *Manager) newReaderConcurrent(file *os.File, fp *Fingerprint) (*Reader, error) {
	// Check if the new path has the same fingerprint as an old path
	if oldReader, ok := m.findFingerprintMatchConcurrent(fp); ok {
		return m.readerFactory.copy(oldReader, file)
	}

	// If we don't match any previously known files, create a new reader from scratch
	return m.readerFactory.newReader(file, fp.Copy())
}

func (m *Manager) findFingerprintMatchConcurrent(fp *Fingerprint) (*Reader, bool) {
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
