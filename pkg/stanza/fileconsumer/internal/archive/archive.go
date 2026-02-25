// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

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

type Archive interface {
	FindFiles(context.Context, []*fingerprint.Fingerprint) []*reader.Metadata
	WriteFiles(context.Context, *fileset.Fileset[*reader.Metadata])
}

func New(ctx context.Context, logger *zap.Logger, pollsToArchive int, persister operator.Persister) Archive {
	if pollsToArchive <= 0 || persister == nil {
		logger.Debug("archiving is disabled. enable pollsToArchive and storage settings to save offsets on disk.")
		return &nopArchive{}
	}

	// restore last known archive index
	archiveIndex, err := getArchiveIndex(ctx, persister)
	switch {
	case err != nil:
		logger.Error("failed to read archive index. Resetting it to 0", zap.Error(err))
		archiveIndex = 0
	case archiveIndex >= pollsToArchive:
		logger.Warn("archiveIndex is out of bounds, likely due to change in pollsToArchive. Resetting it to 0") // Try to craft log to explain in user facing terms?
		archiveIndex = 0
	default:
		// archiveIndex should point to index for the next write, hence increment it from last known value.
		archiveIndex = (archiveIndex + 1) % pollsToArchive
	}
	return &archive{
		pollsToArchive: pollsToArchive,
		persister:      persister,
		archiveIndex:   archiveIndex,
		logger:         logger,
	}
}

type archive struct {
	persister operator.Persister

	pollsToArchive int

	// archiveIndex points to the index for the next write.
	archiveIndex int
	logger       *zap.Logger
}

func (a *archive) FindFiles(ctx context.Context, fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// FindFiles goes through archive, one fileset at a time and tries to match all fingerprints against that loaded set.
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

		data, err := a.readArchive(ctx, nextIndex) // we load one fileset atmost once per poll
		if err != nil {
			a.logger.Error("failed to read archive", zap.Error(err))
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
		if err := a.writeArchive(ctx, nextIndex, data); err != nil {
			a.logger.Error("failed to write archive", zap.Error(err))
		}
		// Check if all metadata have been found
		if numMatched == len(fps) {
			return matchedMetadata
		}
	}
	return matchedMetadata
}

func (a *archive) WriteFiles(ctx context.Context, metadata *fileset.Fileset[*reader.Metadata]) {
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

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(a.archiveIndex); err != nil {
		a.logger.Error("failed to encode archive index", zap.Error(err))
	}
	indexOp := storage.SetOperation(archiveIndexKey, buf.Bytes()) // batch the updated index with metadata
	if err := a.writeArchive(ctx, a.archiveIndex, metadata, indexOp); err != nil {
		a.logger.Error("failed to write archive", zap.Error(err))
	}
	a.archiveIndex = (a.archiveIndex + 1) % a.pollsToArchive
}

func (a *archive) readArchive(ctx context.Context, index int) (*fileset.Fileset[*reader.Metadata], error) {
	// readArchive loads data from the archive for a given index and returns a fileset.Filset.
	metadata, err := checkpoint.LoadKey(ctx, a.persister, archiveKey(index))
	if err != nil {
		return nil, err
	}
	f := fileset.New[*reader.Metadata](len(metadata))
	f.Add(metadata...)
	return f, nil
}

func (a *archive) writeArchive(ctx context.Context, index int, rmds *fileset.Fileset[*reader.Metadata], ops ...*storage.Operation) error {
	// writeArchive saves data to the archive for a given index and returns an error, if encountered.
	return checkpoint.SaveKey(ctx, a.persister, rmds.Get(), archiveKey(index), ops...)
}

func getArchiveIndex(ctx context.Context, persister operator.Persister) (int, error) {
	byteIndex, err := persister.Get(ctx, archiveIndexKey)
	if err != nil {
		return 0, err
	}
	var archiveIndex int
	if err := json.NewDecoder(bytes.NewReader(byteIndex)).Decode(&archiveIndex); err != nil {
		return 0, err
	}
	return archiveIndex, nil
}

func archiveKey(i int) string {
	return fmt.Sprintf("knownFiles%d", i)
}

type nopArchive struct{}

func (*nopArchive) FindFiles(_ context.Context, fps []*fingerprint.Fingerprint) []*reader.Metadata {
	// we return an array of "nil"s, indicating 0 matches are found in archive
	return make([]*reader.Metadata, len(fps))
}

func (*nopArchive) WriteFiles(context.Context, *fileset.Fileset[*reader.Metadata]) {
}
