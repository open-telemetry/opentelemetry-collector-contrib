package fileconsumer

import (
	"context"
	"os"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

	"go.uber.org/zap"
)

// following methods are only applicable to threadpool
func (m *Manager) worker(ctx context.Context, queue chan readerEnvelope) {
	for {
		select {
		case wrapper, ok := <-queue:
			if !ok {
				return
			}
			r, fp := wrapper.reader, wrapper.trieKey
			r.ReadToEnd(ctx)
			// Save off any files that were not fully read.
			m.knownFilesLock.Lock()
			m.previousPollFiles = append(m.previousPollFiles, r)
			m.knownFilesLock.Unlock()
			m.trieDelete(fp)
		}
	}
}

func (m *Manager) workerLostReaders(ctx context.Context, queue chan readerEnvelope) {
	for {
		select {
		case wrapper, ok := <-queue:
			if !ok {
				return
			}
			r, fp := wrapper.reader, wrapper.trieKey
			r.ReadToEnd(ctx)
			r.Close()
			m.trieDelete(fp)
		}
	}
}

func (m *Manager) makeReadersConcurrent(paths []string) []*reader.Reader {
	readers := make([]*reader.Reader, 0, len(paths))
	for _, path := range paths {
		fp, file := m.makeFingerprint(path)
		if fp == nil {
			continue
		}
		if m.isCurrentlyConsuming(fp) {
			continue
		}

		// Exclude duplicate paths with the same content. This can happen when files are
		// being rotated with copy/truncate strategy. (After copy, prior to truncate.)
		for _, r := range readers {
			if fp.Equal(r.Fingerprint) {
				if err := file.Close(); err != nil {
					m.Debugw("problem closing file", zap.Error(err))
				}
				continue
			}
		}

		r, err := m.newReaderConcurrent(file, fp)
		if err != nil {
			m.Errorw("Failed to create reader", zap.Error(err))
			continue
		}

		readers = append(readers, r)
	}
	return readers

}

func (m *Manager) consumeConcurrent(ctx context.Context, paths []string) {
	m.Debug("Consuming files")
	m.clearOldReadersConcurrent(ctx)
	readers := m.makeReadersConcurrent(paths)
	for _, r := range readers {
		// add fingerprint to trie
		fp := r.Metadata.Fingerprint.Copy()
		m.triePut(fp)
		m.pool.Submit(readerEnvelope{reader: r, trieKey: fp})
	}
}

func (m *Manager) isCurrentlyConsuming(fp *fingerprint.Fingerprint) bool {
	m.trieLock.RLock()
	defer m.trieLock.RUnlock()
	return m.trie.Get(fp.FirstBytes)
}

func (m *Manager) triePut(fp *fingerprint.Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Put(fp.FirstBytes, true)
}

func (m *Manager) trieDelete(fp *fingerprint.Fingerprint) {
	m.trieLock.Lock()
	defer m.trieLock.Unlock()
	m.trie.Delete(fp.FirstBytes)
}

func (m *Manager) clearOldReadersConcurrent(ctx context.Context) {
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	var wg sync.WaitGroup
	for _, r := range m.previousPollFiles {
		if r.Validate() {
			// m.triePut(r.Fingerprint)
			// m.poolLost.Submit(readerEnvelope{reader: r, trieKey: r.Fingerprint})
			wg.Add(1)
			go func(r *reader.Reader) {
				defer wg.Done()
				r.ReadToEnd(ctx)
				r.Close()
			}(r)
		}
	}
	wg.Wait()
	m.closePreviousFiles()
}

func (m *Manager) newReaderConcurrent(file *os.File, fp *fingerprint.Fingerprint) (*reader.Reader, error) {
	m.knownFilesLock.Lock()
	defer m.knownFilesLock.Unlock()
	// If we don't match any previously known files, create a new reader from scratch
	return m.newReader(file, fp.Copy())
}

// syncLastPollFiles syncs the most recent set of files to the database
func (m *Manager) syncLastPollFilesConcurrent(ctx context.Context) {
	m.knownFilesLock.RLock()
	defer m.knownFilesLock.RUnlock()

	if m.persister != nil {
		if err := checkpoint.Save(context.Background(), m.persister, m.knownFiles); err != nil {
			m.Errorw("save offsets", zap.Error(err))
		}
	}
}

// poll checks all the watched paths for new entries
func (m *Manager) pollConcurrent(ctx context.Context) {
	// Increment the generation on all known readers
	// This is done here because the next generation is about to start
	m.knownFilesLock.Lock()
	// for i := 0; i < len(m.knownFiles); i++ {
	// 	m.knownFiles[i].generation++
	// }
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
	m.readerFactory.FromBeginning = true
	m.syncLastPollFilesConcurrent(ctx)
}
