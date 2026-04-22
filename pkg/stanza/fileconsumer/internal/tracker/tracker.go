// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"context"
	"os"
	"slices"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

const (
	FileTracker    = "fileTracker"
	NoStateTracker = "noStateTracker"
)

// Interface for tracking files that are being consumed.
type Tracker interface {
	Name() string
	Add(reader *reader.Reader)
	GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader
	GetOpenFile(fp *fingerprint.Fingerprint) *reader.Reader
	GetClosedFile(fp *fingerprint.Fingerprint) *reader.Metadata
	GetMetadata() []*reader.Metadata
	LoadMetadata(metadata []*reader.Metadata)
	CurrentPollFiles() []*reader.Reader
	PreviousPollFiles() []*reader.Reader
	ClosePreviousFiles() int
	EndPoll(context.Context)
	EndConsume() int
	TotalReaders() int
	AddUnmatched(*os.File, *fingerprint.Fingerprint)
	LookupArchive(context.Context) ([]*os.File, []*fingerprint.Fingerprint, []*reader.Metadata)

	// TryReuseByPathMtime is called before opening and fingerprinting a path.
	// If a previously tracked reader has LastObservedPath == path and
	// LastObservedMtime == mtime, its metadata is promoted into the current
	// generation (so it ages on the same schedule as if it had been processed)
	// and the method returns true to signal the caller to skip this path.
	// Callers should only invoke this when the filelog.skipUnchangedPathByMtime
	// feature gate is enabled.
	TryReuseByPathMtime(path string, mtime time.Time) bool
}

// fileTracker tracks known offsets for files that are being consumed by the manager.
type fileTracker struct {
	set component.TelemetrySettings

	maxBatchFiles int

	currentPollFiles  *fileset.Fileset[*reader.Reader]
	previousPollFiles *fileset.Fileset[*reader.Reader]
	knownFiles        []*fileset.Fileset[*reader.Metadata]

	unmatchedFiles []*os.File
	unmatchedFps   []*fingerprint.Fingerprint

	archive archive.Archive
}

func NewFileTracker(ctx context.Context, set component.TelemetrySettings, maxBatchFiles, pollsToArchive int, persister operator.Persister) Tracker {
	knownFiles := make([]*fileset.Fileset[*reader.Metadata], 3)
	for i := range knownFiles {
		knownFiles[i] = fileset.New[*reader.Metadata](maxBatchFiles)
	}
	set.Logger = set.Logger.With(zap.String("tracker", "fileTracker"))

	t := &fileTracker{
		set:               set,
		maxBatchFiles:     maxBatchFiles,
		currentPollFiles:  fileset.New[*reader.Reader](maxBatchFiles),
		previousPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
		knownFiles:        knownFiles,
		archive:           archive.New(ctx, set.Logger.Named("archive"), pollsToArchive, persister),
	}
	return t
}

func (*fileTracker) Name() string {
	return FileTracker
}

func (t *fileTracker) Add(reader *reader.Reader) {
	// add a new reader for tracking
	t.currentPollFiles.Add(reader)
}

func (t *fileTracker) GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.currentPollFiles.MatchEqual(fp)
}

func (t *fileTracker) GetOpenFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.previousPollFiles.MatchStartsWith(fp)
}

func (t *fileTracker) GetClosedFile(fp *fingerprint.Fingerprint) *reader.Metadata {
	for i := 0; i < len(t.knownFiles); i++ {
		if oldMetadata := t.knownFiles[i].MatchStartsWith(fp); oldMetadata != nil {
			return oldMetadata
		}
	}
	return nil
}

func (t *fileTracker) AddUnmatched(file *os.File, fp *fingerprint.Fingerprint) {
	// exclude duplicate fingerprints
	if slices.ContainsFunc(t.unmatchedFps, fp.Equal) {
		t.set.Logger.Debug("Skipping duplicate file", zap.String("path", file.Name()))
		return
	}
	t.unmatchedFps = append(t.unmatchedFps, fp)
	t.unmatchedFiles = append(t.unmatchedFiles, file)
}

func (t *fileTracker) LookupArchive(ctx context.Context) ([]*os.File, []*fingerprint.Fingerprint, []*reader.Metadata) {
	// LookupArchive performs fingerprint matching against the archive and returns matched metadata, files and fingerprints.
	metadata := t.archive.FindFiles(ctx, t.unmatchedFps)
	return t.unmatchedFiles, t.unmatchedFps, metadata
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
	return filesClosed
}

func (t *fileTracker) EndPoll(ctx context.Context) {
	// shift the filesets at end of every poll() call
	// t.knownFiles[0] -> t.knownFiles[1] -> t.knownFiles[2]

	// Instead of throwing it away, archive it.
	oldest := t.knownFiles[len(t.knownFiles)-1]
	t.archive.WriteFiles(ctx, oldest)
	copy(t.knownFiles[1:], t.knownFiles[:len(t.knownFiles)-1])
	oldest.Reset()
	t.knownFiles[0] = oldest
}

func (t *fileTracker) TotalReaders() int {
	total := t.previousPollFiles.Len()
	for i := 0; i < len(t.knownFiles); i++ {
		total += t.knownFiles[i].Len()
	}
	return total
}

// TryReuseByPathMtime searches previousPollFiles and every knownFiles
// generation for a reader whose LastObservedPath and LastObservedMtime match
// the given arguments. On match, the metadata is removed from wherever it was
// found and re-added to knownFiles[0] so it ages in lock-step with files that
// were processed this poll. A matching previous-poll reader is closed (its
// file handle released) and only its Metadata is promoted. Returns true iff a
// match was found.
//
// Intentionally does NOT consult the internal archive: that store's purpose
// is fingerprint-based resume-identity for readers that have aged out of the
// active generation window, and path+mtime lookup there would conflate
// path-based identity with fingerprint-based identity (e.g. an archived
// pre-rotation entry could share both path and mtime with a current
// post-rotation file).
func (t *fileTracker) TryReuseByPathMtime(path string, mtime time.Time) bool {
	if path == "" {
		return false
	}

	// previousPollFiles: Readers kept across polls for reread detection.
	if idx := indexOfPathMtimeReader(t.previousPollFiles.Get(), path, mtime); idx >= 0 {
		matched := t.previousPollFiles.Get()[idx]
		t.previousPollFiles.RemoveAt(idx)
		// Release the file handle and retain only Metadata; promote to knownFiles[0].
		t.knownFiles[0].Add(matched.Close())
		return true
	}

	// knownFiles generations. If found in knownFiles[0] we just leave it in
	// place (already in the freshest generation). Otherwise we remove from the
	// older generation and re-add to [0] so it ages fresh from here.
	for i := 0; i < len(t.knownFiles); i++ {
		idx := indexOfPathMtimeMetadata(t.knownFiles[i].Get(), path, mtime)
		if idx < 0 {
			continue
		}
		if i == 0 {
			return true
		}
		matched := t.knownFiles[i].Get()[idx]
		t.knownFiles[i].RemoveAt(idx)
		t.knownFiles[0].Add(matched)
		return true
	}

	return false
}

// indexOfPathMtimeReader finds the first Reader whose Metadata matches the
// path+mtime pair, or -1 if none.
func indexOfPathMtimeReader(rs []*reader.Reader, path string, mtime time.Time) int {
	for i, r := range rs {
		if r.Metadata == nil {
			continue
		}
		if r.LastObservedPath == path && r.LastObservedMtime.Equal(mtime) {
			return i
		}
	}
	return -1
}

// indexOfPathMtimeMetadata finds the first Metadata matching the path+mtime
// pair, or -1 if none.
func indexOfPathMtimeMetadata(ms []*reader.Metadata, path string, mtime time.Time) int {
	for i, m := range ms {
		if m == nil {
			continue
		}
		if m.LastObservedPath == path && m.LastObservedMtime.Equal(mtime) {
			return i
		}
	}
	return -1
}

// noStateTracker only tracks the current polled files. Once the poll is
// complete and telemetry is consumed, the tracked files are closed. The next
// poll will create fresh readers with no previously tracked offsets.
type noStateTracker struct {
	set              component.TelemetrySettings
	maxBatchFiles    int
	currentPollFiles *fileset.Fileset[*reader.Reader]
	unmatchedFiles   []*os.File
	unmatchedFps     []*fingerprint.Fingerprint
}

func NewNoStateTracker(set component.TelemetrySettings, maxBatchFiles int) Tracker {
	set.Logger = set.Logger.With(zap.String("tracker", "noStateTracker"))
	return &noStateTracker{
		set:              set,
		maxBatchFiles:    maxBatchFiles,
		currentPollFiles: fileset.New[*reader.Reader](maxBatchFiles),
	}
}

func (*noStateTracker) Name() string {
	return NoStateTracker
}

func (t *noStateTracker) Add(reader *reader.Reader) {
	// add a new reader for tracking
	t.currentPollFiles.Add(reader)
}

func (t *noStateTracker) CurrentPollFiles() []*reader.Reader {
	return t.currentPollFiles.Get()
}

func (t *noStateTracker) GetCurrentFile(fp *fingerprint.Fingerprint) *reader.Reader {
	return t.currentPollFiles.MatchEqual(fp)
}

func (t *noStateTracker) EndConsume() (filesClosed int) {
	for r, _ := t.currentPollFiles.Pop(); r != nil; r, _ = t.currentPollFiles.Pop() {
		r.Close()
		filesClosed++
	}
	t.unmatchedFiles = t.unmatchedFiles[:0]
	t.unmatchedFps = t.unmatchedFps[:0]
	return filesClosed
}

func (*noStateTracker) GetOpenFile(*fingerprint.Fingerprint) *reader.Reader { return nil }

func (*noStateTracker) GetClosedFile(*fingerprint.Fingerprint) *reader.Metadata { return nil }

func (*noStateTracker) GetMetadata() []*reader.Metadata { return nil }

func (*noStateTracker) LoadMetadata([]*reader.Metadata) {}

func (*noStateTracker) PreviousPollFiles() []*reader.Reader { return nil }

func (*noStateTracker) ClosePreviousFiles() int { return 0 }

func (*noStateTracker) EndPoll(context.Context) {}

func (*noStateTracker) TotalReaders() int { return 0 }

func (t *noStateTracker) AddUnmatched(file *os.File, fp *fingerprint.Fingerprint) {
	t.unmatchedFiles = append(t.unmatchedFiles, file)
	t.unmatchedFps = append(t.unmatchedFps, fp)
}

func (t *noStateTracker) LookupArchive(context.Context) ([]*os.File, []*fingerprint.Fingerprint, []*reader.Metadata) {
	return t.unmatchedFiles, t.unmatchedFps, make([]*reader.Metadata, len(t.unmatchedFps))
}

// TryReuseByPathMtime always returns false for noStateTracker; there is no
// persistent state to reuse across polls.
func (*noStateTracker) TryReuseByPathMtime(string, time.Time) bool { return false }
