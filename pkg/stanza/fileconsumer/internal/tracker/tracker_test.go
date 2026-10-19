// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestFileTrackerReusesFilesetsBetweenPolls(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	persister := testutil.NewUnscopedMockPersister()

	tracker := NewFileTracker(t.Context(), set, 10, 0, persister)
	ft := tracker.(*fileTracker)

	firstCurrent := ft.currentPollFiles
	firstPrevious := ft.previousPollFiles

	ft.Add(newTestReader("test1"))
	ft.EndConsume()

	require.Same(t, firstCurrent, ft.previousPollFiles)
	require.Same(t, firstPrevious, ft.currentPollFiles)
	require.Zero(t, ft.currentPollFiles.Len())

	ft.Add(newTestReader("test2"))
	ft.EndConsume()

	require.Same(t, firstPrevious, ft.previousPollFiles)
	require.Same(t, firstCurrent, ft.currentPollFiles)
	require.Zero(t, ft.currentPollFiles.Len())
}

func TestFileTrackerReusesUnmatchedSlices(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	persister := testutil.NewUnscopedMockPersister()

	tracker := NewFileTracker(t.Context(), set, 10, 0, persister)
	ft := tracker.(*fileTracker)

	ft.AddUnmatched(nil, fingerprint.New([]byte("fp1")))
	ft.AddUnmatched(nil, fingerprint.New([]byte("fp2")))

	filesCap := cap(ft.unmatchedFiles)
	fpsCap := cap(ft.unmatchedFps)

	ft.EndConsume()

	require.Empty(t, ft.unmatchedFiles)
	require.Empty(t, ft.unmatchedFps)
	require.Equal(t, filesCap, cap(ft.unmatchedFiles))
	require.Equal(t, fpsCap, cap(ft.unmatchedFps))
}

func TestFileTrackerReusesKnownFilesAcrossPolls(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	persister := testutil.NewUnscopedMockPersister()

	tracker := NewFileTracker(t.Context(), set, 10, 0, persister)
	ft := tracker.(*fileTracker)

	first := ft.knownFiles[0]
	second := ft.knownFiles[1]
	third := ft.knownFiles[2]

	ft.knownFiles[0].Add(newTestReader("known1").Metadata)
	ft.EndPoll(t.Context())

	require.Same(t, third, ft.knownFiles[0])
	require.Same(t, first, ft.knownFiles[1])
	require.Same(t, second, ft.knownFiles[2])
	require.Zero(t, ft.knownFiles[0].Len())
}

func TestNoStateTrackerReusesUnmatchedSlices(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()

	tracker := NewNoStateTracker(set, 10)
	nst := tracker.(*noStateTracker)

	nst.AddUnmatched(nil, fingerprint.New([]byte("fp1")))
	nst.AddUnmatched(nil, fingerprint.New([]byte("fp2")))

	filesCap := cap(nst.unmatchedFiles)
	fpsCap := cap(nst.unmatchedFps)

	nst.EndConsume()

	require.Empty(t, nst.unmatchedFiles)
	require.Empty(t, nst.unmatchedFps)
	require.Equal(t, filesCap, cap(nst.unmatchedFiles))
	require.Equal(t, fpsCap, cap(nst.unmatchedFps))
}

func newTestReader(content string) *reader.Reader {
	return &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte(content)),
		},
	}
}

func TestTryReuseByPathMtime_KnownFilesMatch_PromotesToGenerationZero(t *testing.T) {
	tr := newTestFileTracker(t)

	path := "/var/log/one.log"
	mtime := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	md := &reader.Metadata{
		Fingerprint:       fingerprint.New([]byte("abc")),
		LastObservedPath:  path,
		LastObservedMtime: mtime,
	}
	// Place it in the 2nd-freshest generation (simulates a file that wasn't
	// seen last poll but was seen the poll before).
	tr.knownFiles[1].Add(md)

	ok := tr.TryReuseByPathMtime(path, mtime)

	assert.True(t, ok, "expected a match to be found")
	assert.Equal(t, 0, tr.knownFiles[1].Len(), "metadata should have been removed from knownFiles[1]")
	assert.Equal(t, 1, tr.knownFiles[0].Len(), "metadata should have been promoted to knownFiles[0]")
}

func TestTryReuseByPathMtime_AlreadyInGenerationZero_NoMovement(t *testing.T) {
	tr := newTestFileTracker(t)

	path := "/var/log/fresh.log"
	mtime := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	md := &reader.Metadata{
		Fingerprint:       fingerprint.New([]byte("fresh")),
		LastObservedPath:  path,
		LastObservedMtime: mtime,
	}
	tr.knownFiles[0].Add(md)

	ok := tr.TryReuseByPathMtime(path, mtime)

	assert.True(t, ok)
	assert.Equal(t, 1, tr.knownFiles[0].Len(), "metadata should still be exactly once in knownFiles[0]")
}

func TestTryReuseByPathMtime_PathMatchesButMtimeDiffers_ReturnsFalse(t *testing.T) {
	tr := newTestFileTracker(t)

	path := "/var/log/changed.log"
	mtime := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	md := &reader.Metadata{
		Fingerprint:       fingerprint.New([]byte("x")),
		LastObservedPath:  path,
		LastObservedMtime: mtime,
	}
	tr.knownFiles[0].Add(md)

	// Same path, different mtime.
	ok := tr.TryReuseByPathMtime(path, mtime.Add(1*time.Second))

	assert.False(t, ok)
	assert.Equal(t, 1, tr.knownFiles[0].Len(), "metadata should not have been removed on non-match")
}

func TestTryReuseByPathMtime_EmptyPath_ReturnsFalse(t *testing.T) {
	tr := newTestFileTracker(t)

	ok := tr.TryReuseByPathMtime("", time.Now())

	assert.False(t, ok)
}

func TestTryReuseByPathMtime_NoMatch_ReturnsFalse(t *testing.T) {
	tr := newTestFileTracker(t)

	md := &reader.Metadata{
		Fingerprint:       fingerprint.New([]byte("y")),
		LastObservedPath:  "/var/log/some.log",
		LastObservedMtime: time.Now(),
	}
	tr.knownFiles[2].Add(md)

	ok := tr.TryReuseByPathMtime("/var/log/other.log", time.Now())

	assert.False(t, ok)
}

func TestTryReuseByPathMtime_NoStateTracker_AlwaysFalse(t *testing.T) {
	tr := NewNoStateTracker(componenttest.NewNopTelemetrySettings(), 10)

	assert.False(t, tr.TryReuseByPathMtime("/any", time.Now()))
}

// newTestFileTracker returns a concrete *fileTracker so tests can inspect
// knownFiles and previousPollFiles directly. It skips the archive so there's
// no persister requirement.
func newTestFileTracker(t *testing.T) *fileTracker {
	t.Helper()
	tr := NewFileTracker(t.Context(), componenttest.NewNopTelemetrySettings(), 10, 0, nil)
	ft, ok := tr.(*fileTracker)
	require.True(t, ok, "NewFileTracker should return *fileTracker")
	return ft
}
