// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const (
	archiveIndexKey          = "knownFilesArchiveIndex"
	archivePollsToArchiveKey = "knonwFilesPollsToArchive"
)

type Archive interface {
	FindFiles([]*fingerprint.Fingerprint) []*reader.Metadata
	WriteFiles(*fileset.Fileset[*reader.Metadata])
}

func NewArchive(ctx context.Context, logger *zap.Logger, pollsToArchive int, persister operator.Persister) Archive {
	if pollsToArchive > 0 && persister != nil {
		a := &archive{
			pollsToArchive: pollsToArchive,
			persister:      persister,
			archiveIndex:   0,
			logger:         logger,
		}
		a.restoreArchiveIndex(ctx)
		return a
	} else {
		return &nopArchive{}
	}
}

type archive struct {
	// persister is to be used to store offsets older than 3 poll cycles.
	// These offsets will be stored on disk
	persister operator.Persister

	pollsToArchive int
	archiveIndex   int
	logger         *zap.Logger
}

// FindFiles goes through archive, one fileset at a time and tries to match all fingerprints against that loaded set.
func (a *archive) FindFiles(fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// To minimize disk access, we first access the index, then review unmatched files and update the metadata, if found.
	// We exit if all fingerprints are matched.

	// Track number of matched fingerprints so we can exit if all matched.
	var numMatched int

	// Determine the index for reading archive, starting from the most recent and moving towards the oldest
	nextIndex := a.archiveIndex
	matchedMetadata := make([]*reader.Metadata, len(fps))

	// continue executing the loop until either all records are matched or all archive sets have been processed.
	for i := 0; i < a.pollsToArchive; i++ {
		// Update the mostRecentIndex
		nextIndex = (nextIndex - 1 + a.pollsToArchive) % a.pollsToArchive

		data, err := a.readArchive(nextIndex) // we load one fileset atmost once per poll
		if err != nil {
			a.logger.Error("error while opening archive", zap.Error(err))
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
		if err := a.writeArchive(nextIndex, data); err != nil {
			a.logger.Error("error while opening archive", zap.Error(err))
		}
		// Check if all metadata have been found
		if numMatched == len(fps) {
			return matchedMetadata
		}
	}
	return matchedMetadata
}

func (a *archive) WriteFiles(metadata *fileset.Fileset[*reader.Metadata]) {
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

	index := a.archiveIndex
	a.archiveIndex = (a.archiveIndex + 1) % a.pollsToArchive                      // increment the index
	indexOp := storage.SetOperation(archiveIndexKey, encodeIndex(a.archiveIndex)) // batch the updated index with metadata
	if err := a.writeArchive(index, metadata, indexOp); err != nil {
		a.logger.Error("error faced while saving to the archive", zap.Error(err))
	}
}

// readArchive loads data from the archive for a given index and returns a fileset.Filset.
func (a *archive) readArchive(index int) (*fileset.Fileset[*reader.Metadata], error) {
	metadata, err := checkpoint.LoadKey(context.Background(), a.persister, archiveKey(index))
	if err != nil {
		return nil, err
	}
	f := fileset.New[*reader.Metadata](len(metadata))
	f.Add(metadata...)
	return f, nil
}

// writeArchive saves data to the archive for a given index and returns an error, if encountered.
func (a *archive) writeArchive(index int, rmds *fileset.Fileset[*reader.Metadata], ops ...*storage.Operation) error {
	return checkpoint.SaveKey(context.Background(), a.persister, rmds.Get(), archiveKey(index), ops...)
}

func (a *archive) restoreArchiveIndex(ctx context.Context) {
	var err error
	// remove extra "keys" in case `pollsToArchive` has changed between collector restarts
	defer a.removeExtraKeys(ctx)

	a.archiveIndex, err = a.getArchiveIndex(ctx)
	if err != nil {
		a.logger.Error("error while fetching archive index", zap.Error(err))
		a.archiveIndex = 0
	}
	if a.archiveIndex >= a.pollsToArchive || a.archiveIndex < 0 {
		// archiveIndex is out of bounds. This most likely happened if `pollsToArchive` changed between collector restarts
		// we just set archiveIndex to 0 in this case i.e. to reboot the archive index
		a.archiveIndex = 0
	}
}

func (a *archive) removeExtraKeys(ctx context.Context) {
	for i := a.pollsToArchive; a.isSet(ctx, i); i++ {
		if err := a.persister.Delete(ctx, archiveKey(i)); err != nil {
			a.logger.Error("error while cleaning extra keys", zap.Error(err))
		}
	}
}

func (a *archive) getArchiveIndex(ctx context.Context) (int, error) {
	byteIndex, err := a.persister.Get(ctx, archiveIndexKey)
	if err != nil {
		return 0, err
	}
	archiveIndex, err := decodeIndex(byteIndex)
	if err != nil {
		return 0, err
	}
	return archiveIndex, nil
}

// isSet returns true of index `i` has data present in the archive ring buffer
func (a *archive) isSet(ctx context.Context, i int) bool {
	val, err := a.persister.Get(ctx, archiveKey(i))
	return val != nil && err == nil
}

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

type nopArchive struct{}

func (*nopArchive) FindFiles(fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// we return an array of "nil"s, indicating 0 matches are found in archive
	return make([]*reader.Metadata, len(fps))
}

func (*nopArchive) WriteFiles(metadata *fileset.Fileset[*reader.Metadata]) {
}
