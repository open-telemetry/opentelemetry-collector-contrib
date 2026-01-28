// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestFileTrackerMemoryReuse(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	persister := testutil.NewUnscopedMockPersister()
	ctx := t.Context()

	tracker := NewFileTracker(ctx, set, 10, 0, persister)
	ft := tracker.(*fileTracker)

	// First cycle: add and end
	r1 := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte("test1")),
		},
	}
	ft.Add(r1)
	firstCurrent := ft.currentPollFiles
	ft.EndConsume()

	// After first EndConsume, previous should be the first current
	require.Equal(t, firstCurrent, ft.previousPollFiles)

	// Second cycle: add and end
	r2 := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte("test2")),
		},
	}
	ft.Add(r2)
	ft.EndConsume()

	// After second EndConsume, current should reuse first current (from old previous)
	require.Equal(t, firstCurrent, ft.currentPollFiles)
	require.Equal(t, 0, ft.currentPollFiles.Len(), "Reused fileset should be empty")

	// Third cycle: verify reused fileset works
	r3 := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte("test3")),
		},
	}
	ft.Add(r3)
	require.Equal(t, 1, ft.currentPollFiles.Len())
}

func TestFileTrackerUnmatchedSliceReuse(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	persister := testutil.NewUnscopedMockPersister()
	ctx := t.Context()

	tracker := NewFileTracker(ctx, set, 10, 0, persister)
	ft := tracker.(*fileTracker)

	fp1 := fingerprint.New([]byte("fp1"))
	ft.unmatchedFps = append(ft.unmatchedFps, fp1)
	ft.unmatchedFiles = append(ft.unmatchedFiles, nil) // File pointer doesn't matter for this test

	fpCapBefore := cap(ft.unmatchedFps)
	filesCapBefore := cap(ft.unmatchedFiles)

	ft.EndConsume()

	require.Empty(t, ft.unmatchedFps)
	require.Empty(t, ft.unmatchedFiles)
	require.Equal(t, fpCapBefore, cap(ft.unmatchedFps))
	require.Equal(t, filesCapBefore, cap(ft.unmatchedFiles))
}

func TestNoStateTrackerMemoryReuse(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	tracker := NewNoStateTracker(set, 10)
	nst := tracker.(*noStateTracker)

	r1 := &reader.Reader{
		Metadata: &reader.Metadata{
			Fingerprint: fingerprint.New([]byte("test1")),
		},
	}
	nst.Add(r1)

	filesClosed := nst.EndConsume()
	require.Equal(t, 1, filesClosed)

	require.Empty(t, nst.unmatchedFiles)
	require.Empty(t, nst.unmatchedFps)
}
