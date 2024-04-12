// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

type Tracker struct {
	*zap.SugaredLogger

	maxBatchFiles int

	currentPollFiles  *fileset.Fileset[*reader.Reader]
	previousPollFiles *fileset.Fileset[*reader.Reader]
	knownFiles        []*fileset.Fileset[*reader.Metadata]
}

func New(logger *zap.SugaredLogger, maxBatchFiles int) *Tracker {
	knownFiles := make([]*fileset.Fileset[*reader.Metadata], 3)
	for i := 0; i < len(knownFiles); i++ {
		knownFiles[i] = fileset.New[*reader.Metadata](maxBatchFiles)
	}
	return &Tracker{
		SugaredLogger:     logger.With("component", "fileconsumer"),
		maxBatchFiles:     maxBatchFiles,
		currentPollFiles:  fileset.New[*reader.Reader](maxBatchFiles),
		previousPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
		knownFiles:        knownFiles,
	}
}

func (t *Tracker) Add(reader *reader.Reader) {
	// add a new reader for tracking
	t.currentPollFiles.Add(reader)
}

func (t *Tracker) GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.currentPollFiles.Match(fp, fileset.Equal)
}

func (t *Tracker) GetOpenFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.previousPollFiles.Match(fp, fileset.StartsWith)
}

func (t *Tracker) GetClosedFile(fp *fingerprint.Fingerprint) *reader.Metadata {
	for i := 0; i < len(t.knownFiles); i++ {
		if oldMetadata := t.knownFiles[i].Match(fp, fileset.StartsWith); oldMetadata != nil {
			return oldMetadata
		}
	}
	return nil
}

func (t *Tracker) GetMetadata() []*reader.Metadata {
	// return all known metadata for checkpoining
	allCheckpoints := make([]*reader.Metadata, 0, t.TotalReaders())
	for _, knownFiles := range t.knownFiles {
		allCheckpoints = append(allCheckpoints, knownFiles.Get()...)
	}

	for _, r := range t.previousPollFiles.Get() {
		allCheckpoints = append(allCheckpoints, r.Metadata)
	}
	return allCheckpoints
}

func (t *Tracker) LoadMetadata(metadata []*reader.Metadata) {
	t.knownFiles[0].Add(metadata...)
}

func (t *Tracker) CurrentPollFiles() []*reader.Reader {
	return t.currentPollFiles.Get()
}

func (t *Tracker) PreviousPollFiles() []*reader.Reader {
	return t.previousPollFiles.Get()
}

func (t *Tracker) ClosePreviousFiles() {
	// t.previousPollFiles -> t.knownFiles[0]

	for r, _ := t.previousPollFiles.Pop(); r != nil; r, _ = t.previousPollFiles.Pop() {
		t.knownFiles[0].Add(r.Close())
	}
}

func (t *Tracker) EndPoll() {
	// shift the filesets at end of every poll() call
	// t.knownFiles[0] -> t.knownFiles[1] -> t.knownFiles[2]
	copy(t.knownFiles[1:], t.knownFiles)
	t.knownFiles[0] = fileset.New[*reader.Metadata](t.maxBatchFiles)
}

func (t *Tracker) TotalReaders() int {
	total := t.previousPollFiles.Len()
	for i := 0; i < len(t.knownFiles); i++ {
		total += t.knownFiles[i].Len()
	}
	return total
}
