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
