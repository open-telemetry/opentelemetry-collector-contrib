// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const (
	archiveIndexKey          = "knownFilesArchiveIndex"
	archivePollsToArchiveKey = "knonwFilesPollsToArchive"
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

func NewFileTracker(ctx context.Context, set component.TelemetrySettings, maxBatchFiles int, pollsToArchive int, persister operator.Persister) Tracker {
	knownFiles := make([]*fileset.Fileset[*reader.Metadata], 3)
	for i := 0; i < len(knownFiles); i++ {
		knownFiles[i] = fileset.New[*reader.Metadata](maxBatchFiles)
	}
	set.Logger = set.Logger.With(zap.String("tracker", "fileTracker"))

	t := &fileTracker{
		set:               set,
		maxBatchFiles:     maxBatchFiles,
		currentPollFiles:  fileset.New[*reader.Reader](maxBatchFiles),
		previousPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
		knownFiles:        knownFiles,
		pollsToArchive:    pollsToArchive,
		persister:         persister,
		archiveIndex:      0,
	}
	if t.archiveEnabled() {
		t.restoreArchiveIndex(ctx)
	}

	return t
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
	if t.archiveEnabled() {
		t.archive(t.knownFiles[2])
	}
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

func (t *fileTracker) restoreArchiveIndex(ctx context.Context) {
	// remove extra "keys" once archive restoration is done
	defer t.removeExtraKeys(ctx)
	defer func() {
		// store current pollsToArchive
		if err := t.persister.Set(ctx, archivePollsToArchiveKey, encodeIndex(t.pollsToArchive)); err != nil {
			t.set.Logger.Error("Error storing polls_to_archive", zap.Error(err))
		}
	}()

	previousPollsToArchive, err := t.getPreviousPollsToArchive(ctx)
	if err != nil {
		// if there's an error reading previousPollsToArchive, default to current value
		previousPollsToArchive = t.pollsToArchive
	}

	t.archiveIndex, err = t.getArchiveIndex(ctx)
	if err != nil {
		t.set.Logger.Error("error while reading the archiveIndexKey. Starting from 0", zap.Error(err))
		return
	}

	if previousPollsToArchive < t.pollsToArchive {
		// if archive size has increased, we just increment the index until we enconter a nil value
		for t.archiveIndex < t.pollsToArchive && t.isSet(ctx, t.archiveIndex) {
			t.archiveIndex++
		}
	} else if previousPollsToArchive > t.pollsToArchive {
		// we will only attempt to rewrite archive if the archive size has shrunk
		t.set.Logger.Warn("polls_to_archive has changed. Will attempt to rewrite archive")
		t.rewriteArchive(ctx, previousPollsToArchive)
	}
}

func (t *fileTracker) rewriteArchive(ctx context.Context, previousPollsToArchive int) {
	// helper to rewrite data from oldIndex to newIndex
	rewrite := func(newIdx, oldIdex int) error {
		oldVal, err := t.persister.Get(ctx, archiveKey(oldIdex))
		if err != nil {
			return err
		}
		return t.persister.Set(ctx, archiveKey(newIdx), oldVal)
	}
	// Calculate the least recent index, w.r.t. new archive size

	leastRecentIndex := mod(t.archiveIndex-t.pollsToArchive, previousPollsToArchive)

	// Refer archive.md for the detailed design
	if mod(t.archiveIndex-1, previousPollsToArchive) > t.pollsToArchive {
		for i := 0; i < t.pollsToArchive; i++ {
			if err := rewrite(i, leastRecentIndex); err != nil {
				t.set.Logger.Error("error while swapping archive", zap.Error(err))
			}
			leastRecentIndex = (leastRecentIndex + 1) % previousPollsToArchive
		}
		t.archiveIndex = 0
	} else {
		if !t.isSet(ctx, t.archiveIndex) {
			// If the current index points at an unset key, no need to do anything
			return
		}
		for i := 0; i < t.pollsToArchive-t.archiveIndex; i++ {
			if err := rewrite(t.archiveIndex+i, leastRecentIndex); err != nil {
				t.set.Logger.Warn("error while swapping archive", zap.Error(err))
			}
			leastRecentIndex = (leastRecentIndex + 1) % previousPollsToArchive
		}
	}
}

func (t *fileTracker) removeExtraKeys(ctx context.Context) {
	for i := t.pollsToArchive; t.isSet(ctx, i); i++ {
		if err := t.persister.Delete(ctx, archiveKey(i)); err != nil {
			t.set.Logger.Error("error while cleaning extra keys", zap.Error(err))
		}
	}
}

func (t *fileTracker) getPreviousPollsToArchive(ctx context.Context) (int, error) {
	byteIndex, err := t.persister.Get(ctx, archivePollsToArchiveKey)
	if err != nil {
		t.set.Logger.Error("error while reading the archiveIndexKey", zap.Error(err))
		return 0, err
	}
	previousPollsToArchive, err := decodeIndex(byteIndex)
	if err != nil {
		t.set.Logger.Error("error while decoding previousPollsToArchive", zap.Error(err))
		return 0, err
	}
	return previousPollsToArchive, nil
}

func (t *fileTracker) getArchiveIndex(ctx context.Context) (int, error) {
	byteIndex, err := t.persister.Get(ctx, archiveIndexKey)
	if err != nil {
		return 0, err
	}
	archiveIndex, err := decodeIndex(byteIndex)
	if err != nil {
		return 0, err
	}
	return archiveIndex, nil
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

	index := t.archiveIndex
	t.archiveIndex = (t.archiveIndex + 1) % t.pollsToArchive                      // increment the index
	indexOp := storage.SetOperation(archiveIndexKey, encodeIndex(t.archiveIndex)) // batch the updated index with metadata
	if err := t.writeArchive(index, metadata, indexOp); err != nil {
		t.set.Logger.Error("error faced while saving to the archive", zap.Error(err))
	}
}

// readArchive loads data from the archive for a given index and returns a fileset.Filset.
func (t *fileTracker) readArchive(index int) (*fileset.Fileset[*reader.Metadata], error) {
	metadata, err := checkpoint.LoadKey(context.Background(), t.persister, archiveKey(index))
	if err != nil {
		return nil, err
	}
	f := fileset.New[*reader.Metadata](len(metadata))
	f.Add(metadata...)
	return f, nil
}

// writeArchive saves data to the archive for a given index and returns an error, if encountered.
func (t *fileTracker) writeArchive(index int, rmds *fileset.Fileset[*reader.Metadata], ops ...*storage.Operation) error {
	return checkpoint.SaveKey(context.Background(), t.persister, rmds.Get(), archiveKey(index), ops...)
}

func (t *fileTracker) archiveEnabled() bool {
	return t.pollsToArchive > 0 && t.persister != nil
}

func (t *fileTracker) isSet(ctx context.Context, index int) bool {
	val, err := t.persister.Get(ctx, archiveKey(index))
	return val != nil && err == nil
}

// FindFiles goes through archive, one fileset at a time and tries to match all fingerprints against that loaded set.
func (t *fileTracker) FindFiles(fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// To minimize disk access, we first access the index, then review unmatched files and update the metadata, if found.
	// We exit if all fingerprints are matched.

	// Track number of matched fingerprints so we can exit if all matched.
	var numMatched int

	// Determine the index for reading archive, starting from the most recent and moving towards the oldest
	nextIndex := t.archiveIndex
	matchedMetadata := make([]*reader.Metadata, len(fps))

	// continue executing the loop until either all records are matched or all archive sets have been processed.
	for i := 0; i < t.pollsToArchive; i++ {
		// Update the mostRecentIndex
		nextIndex = (nextIndex - 1 + t.pollsToArchive) % t.pollsToArchive

		data, err := t.readArchive(nextIndex) // we load one fileset atmost once per poll
		if err != nil {
			t.set.Logger.Error("error while opening archive", zap.Error(err))
			continue
		}
		archiveModified := false
		for j, fp := range fps {
			if matchedMetadata[j] != nil {
				// we've already found a match for this index, continue
				continue
			}
			if md := data.Match(fp, fileset.StartsWith); md != nil {
				// update the matched metada for the index
				matchedMetadata[j] = md
				archiveModified = true
				numMatched++
			}
		}
		if !archiveModified {
			continue
		}
		// we save one fileset atmost once per poll
		if err := t.writeArchive(nextIndex, data); err != nil {
			t.set.Logger.Error("error while opening archive", zap.Error(err))
		}
		// Check if all metadata have been found
		if numMatched == len(fps) {
			return matchedMetadata
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

func encodeIndex(val int) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)

	// Encode the index
	if err := enc.Encode(val); err != nil {
		return nil
	}
	return buf.Bytes()
}

func decodeIndex(buf []byte) (int, error) {
	var index int

	// Decode the index
	dec := json.NewDecoder(bytes.NewReader(buf))
	err := dec.Decode(&index)
	return max(index, 0), err
}

func archiveKey(i int) string {
	return fmt.Sprintf("knownFiles%d", i)
}

func mod(x, y int) int {
	return (x + y) % y
}
