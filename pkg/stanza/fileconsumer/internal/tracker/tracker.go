// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// Interface for tracking files that are being consumed.
type Tracker interface {
	Add(reader *reader.Reader)
	GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader
	GetOpenFile(fp *fingerprint.Fingerprint) *reader.Reader
	GetClosedFile(fp *fingerprint.Fingerprint) *reader.Metadata
	GetMetadata() []*reader.Metadata
	LoadMetadata(metadata []*reader.Metadata)
	CurrentPollFiles() []*reader.Reader
	PreviousPollFiles() []*reader.Reader
	ClosePreviousFiles() int
	EndPoll()
	EndConsume() int
	TotalReaders() int
	FindFiles([]*fingerprint.Fingerprint) []*reader.Metadata
}

// fileTracker tracks known offsets for files that are being consumed by the manager.
type fileTracker struct {
	set component.TelemetrySettings

	maxBatchFiles int

	currentPollFiles  *fileset.Fileset[*reader.Reader]
	previousPollFiles *fileset.Fileset[*reader.Reader]
	knownFiles        []*fileset.Fileset[*reader.Metadata]

	// persister is to be used to store offsets older than 3 poll cycles.
	// These offsets will be stored on disk
	persister operator.Persister

	pollsToArchive int
	archiveIndex   int
}

func NewFileTracker(set component.TelemetrySettings, maxBatchFiles int, pollsToArchive int, persister operator.Persister) Tracker {
	knownFiles := make([]*fileset.Fileset[*reader.Metadata], 3)
	for i := 0; i < len(knownFiles); i++ {
		knownFiles[i] = fileset.New[*reader.Metadata](maxBatchFiles)
	}
	set.Logger = set.Logger.With(zap.String("tracker", "fileTracker"))
	return &fileTracker{
		set:               set,
		maxBatchFiles:     maxBatchFiles,
		currentPollFiles:  fileset.New[*reader.Reader](maxBatchFiles),
		previousPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
		knownFiles:        knownFiles,
		pollsToArchive:    pollsToArchive,
		persister:         persister,
		archiveIndex:      0,
	}
}

func (t *fileTracker) Add(reader *reader.Reader) {
	// add a new reader for tracking
	t.currentPollFiles.Add(reader)
}

func (t *fileTracker) GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.currentPollFiles.Match(fp, fileset.Equal)
}

func (t *fileTracker) GetOpenFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.previousPollFiles.Match(fp, fileset.StartsWith)
}

func (t *fileTracker) GetClosedFile(fp *fingerprint.Fingerprint) *reader.Metadata {
	for i := 0; i < len(t.knownFiles); i++ {
		if oldMetadata := t.knownFiles[i].Match(fp, fileset.StartsWith); oldMetadata != nil {
			return oldMetadata
		}
	}
	return nil
}

func (t *fileTracker) GetMetadata() []*reader.Metadata {
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

func (t *fileTracker) LoadMetadata(metadata []*reader.Metadata) {
	t.knownFiles[0].Add(metadata...)
}

func (t *fileTracker) CurrentPollFiles() []*reader.Reader {
	return t.currentPollFiles.Get()
}

func (t *fileTracker) PreviousPollFiles() []*reader.Reader {
	return t.previousPollFiles.Get()
}

func (t *fileTracker) ClosePreviousFiles() (filesClosed int) {
	// t.previousPollFiles -> t.knownFiles[0]
	for r, _ := t.previousPollFiles.Pop(); r != nil; r, _ = t.previousPollFiles.Pop() {
		t.knownFiles[0].Add(r.Close())
		filesClosed++
	}
	return
}

func (t *fileTracker) EndPoll() {
	// shift the filesets at end of every poll() call
	// t.knownFiles[0] -> t.knownFiles[1] -> t.knownFiles[2]

	// Instead of throwing it away, archive it.
	t.archive(t.knownFiles[2])
	copy(t.knownFiles[1:], t.knownFiles)
	t.knownFiles[0] = fileset.New[*reader.Metadata](t.maxBatchFiles)
}

func (t *fileTracker) TotalReaders() int {
	total := t.previousPollFiles.Len()
	for i := 0; i < len(t.knownFiles); i++ {
		total += t.knownFiles[i].Len()
	}
	return total
}

func (t *fileTracker) archive(metadata *fileset.Fileset[*reader.Metadata]) {
	// We make use of a ring buffer, where each set of files is stored under a specific index.
	// Instead of discarding knownFiles[2], write it to the next index and eventually roll over.
	// Separate storage keys knownFilesArchive0, knownFilesArchive1, ..., knownFilesArchiveN, roll over back to knownFilesArchive0

	// Archiving:  ┌─────────────────────on-disk archive─────────────────────────┐
	//             |    ┌───┐     ┌───┐                     ┌──────────────────┐ |
	// index       | ▶  │ 0 │  ▶  │ 1 │  ▶      ...       ▶ │ polls_to_archive │ |
	//             | ▲  └───┘     └───┘                     └──────────────────┘ |
	//             | ▲    ▲                                                ▼     |
	//             | ▲    │ Roll over overriting older offsets, if any     ◀     |
	//             └──────│──────────────────────────────────────────────────────┘
	//                    │
	//                    │
	//                    │
	//                   start
	//                   index

	if t.pollsToArchive <= 0 || t.persister == nil {
		return
	}
	if err := t.writeArchive(t.archiveIndex, metadata); err != nil {
		t.set.Logger.Error("error faced while saving to the archive", zap.Error(err))
	}
	t.archiveIndex = (t.archiveIndex + 1) % t.pollsToArchive // increment the index
}

// readArchive loads data from the archive for a given index and returns a fileset.Filset.
func (t *fileTracker) readArchive(index int) (*fileset.Fileset[*reader.Metadata], error) {
	key := fmt.Sprintf("knownFiles%d", index)
	metadata, err := checkpoint.LoadKey(context.Background(), t.persister, key)
	if err != nil {
		return nil, err
	}
	f := fileset.New[*reader.Metadata](len(metadata))
	f.Add(metadata...)
	return f, nil
}

// writeArchive saves data to the archive for a given index and returns an error, if encountered.
func (t *fileTracker) writeArchive(index int, rmds *fileset.Fileset[*reader.Metadata]) error {
	key := fmt.Sprintf("knownFiles%d", index)
	return checkpoint.SaveKey(context.Background(), t.persister, rmds.Get(), key)
}

// FindFiles goes through archive, one fileset at a time and tries to match all fingerprints against that loaded set.
func (t *fileTracker) FindFiles(fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// To minimize disk access, we first access the index, then review unmatched files and update the metadata, if found.
	// We exit if all fingerprints are matched.

	mostRecentIndex := (t.archiveIndex - 1 + t.pollsToArchive) % t.pollsToArchive
	matchedMetadata := make([]*reader.Metadata, len(fps))

	// continue executing the loop until either all records are matched or all archive sets have been processed.
	for i := 0; i < t.pollsToArchive; i++ {
		metadataUpdated := false

		// Update the mostRecentIndex
		currentIndex := mostRecentIndex
		mostRecentIndex = (mostRecentIndex - 1 + t.pollsToArchive) % t.pollsToArchive

		data, err := t.readArchive(currentIndex) // we load one fileset atmost once per poll
		if err != nil {
			t.set.Logger.Error("error while opening archive", zap.Error(err))
			continue
		}
		for index, fp := range fps {
			if matchedMetadata[index] != nil {
				// we've already found a match for this index, continue
				continue
			}
			if md := data.Match(fp, fileset.StartsWith); md != nil {
				// update the matched metada for the index
				matchedMetadata[index] = md
				metadataUpdated = true
			}
		}
		if metadataUpdated {
			// we save one fileset atmost once per poll
			if err := t.writeArchive(currentIndex, data); err != nil {
				t.set.Logger.Error("error while opening archive", zap.Error(err))
			}
		}
	}
	return matchedMetadata
}

// noStateTracker only tracks the current polled files. Once the poll is
// complete and telemetry is consumed, the tracked files are closed. The next
// poll will create fresh readers with no previously tracked offsets.
type noStateTracker struct {
	set              component.TelemetrySettings
	maxBatchFiles    int
	currentPollFiles *fileset.Fileset[*reader.Reader]
}

func NewNoStateTracker(set component.TelemetrySettings, maxBatchFiles int) Tracker {
	set.Logger = set.Logger.With(zap.String("tracker", "noStateTracker"))
	return &noStateTracker{
		set:              set,
		maxBatchFiles:    maxBatchFiles,
		currentPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
	}
}

func (t *noStateTracker) Add(reader *reader.Reader) {
	// add a new reader for tracking
	t.currentPollFiles.Add(reader)
}

func (t *noStateTracker) CurrentPollFiles() []*reader.Reader {
	return t.currentPollFiles.Get()
}

func (t *noStateTracker) GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.currentPollFiles.Match(fp, fileset.Equal)
}

func (t *noStateTracker) EndConsume() (filesClosed int) {
	for r, _ := t.currentPollFiles.Pop(); r != nil; r, _ = t.currentPollFiles.Pop() {
		r.Close()
		filesClosed++
	}
	return
}

func (t *noStateTracker) GetOpenFile(_ *fingerprint.Fingerprint) *reader.Reader { return nil }

func (t *noStateTracker) GetClosedFile(_ *fingerprint.Fingerprint) *reader.Metadata { return nil }

func (t *noStateTracker) GetMetadata() []*reader.Metadata { return nil }

func (t *noStateTracker) LoadMetadata(_ []*reader.Metadata) {}

func (t *noStateTracker) PreviousPollFiles() []*reader.Reader { return nil }

func (t *noStateTracker) ClosePreviousFiles() int { return 0 }

func (t *noStateTracker) EndPoll() {}

func (t *noStateTracker) TotalReaders() int { return 0 }

func (t *noStateTracker) FindFiles([]*fingerprint.Fingerprint) []*reader.Metadata { return nil }
