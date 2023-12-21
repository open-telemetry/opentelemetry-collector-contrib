// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type Manager struct {
	*zap.SugaredLogger
	tracker *tracker.Tracker
	wg      sync.WaitGroup
	cancel  context.CancelFunc

	fileMatcher *matcher.Matcher

	pollInterval  time.Duration
	persister     operator.Persister
	maxBatches    int
	maxBatchFiles int
}

func (m *Manager) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	if matches, err := m.fileMatcher.MatchFiles(); err != nil {
		m.Warnf("finding files: %v", err)
	} else {
		m.tracker.MovingAverageMatches = len(matches)
	}

	if persister != nil {
		m.persister = persister
		if err := m.tracker.Load(persister); err != nil {
			return err
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
	m.tracker.PostConsume()
	if m.persister != nil {
		m.tracker.Persist(m.persister)
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
	} else {
		m.tracker.MovingAverageMatches = (m.tracker.MovingAverageMatches*3 + len(matches)) / 4
	}
	m.Debugf("matched files", zap.Strings("paths", matches))

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
	m.tracker.FromBeginning()
	if m.persister != nil {
		m.tracker.Persist(m.persister)
	}
}

func (m *Manager) consume(ctx context.Context, paths []string) {
	m.Debug("Consuming files", zap.Strings("paths", paths))
	for _, path := range paths {
		m.tracker.ReadFile(path)
	}

	m.tracker.PreConsume(ctx)

	// read new readers to end
	var wg sync.WaitGroup
	for _, r := range m.tracker.ActiveFiles() {
		wg.Add(1)
		go func(r *reader.Reader) {
			defer wg.Done()
			r.ReadToEnd(ctx)
		}(r)
	}
	wg.Wait()

	m.tracker.PostConsume()
}
