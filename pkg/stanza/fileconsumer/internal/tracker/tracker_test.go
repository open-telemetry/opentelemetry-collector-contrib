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
)

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
