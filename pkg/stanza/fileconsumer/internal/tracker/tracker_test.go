// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/tracker"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/checkpoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestFindFilesOrder(t *testing.T) {
	fps := make([]*fingerprint.Fingerprint, 0)
	for i := 0; i < 100; i++ {
		fps = append(fps, fingerprint.New([]byte(uuid.NewString())))
	}
	persister := testutil.NewUnscopedMockPersister()
	fpInStorage := populatedPersisterData(persister, fps)

	tracker := NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, 100, persister)
	matchables := tracker.FindFiles(fps)

	require.Len(t, matchables, len(fps), "return slice should be of same length as input slice")

	for i := 0; i < len(matchables); i++ {
		if fpInStorage[i] {
			// if current fingerprint is present in storage, the corresponding return type should not be nil
			require.NotNilf(t, matchables[i], "resulting index %d should be not be nil type", i)
			require.Truef(t, fps[i].Equal(matchables[i].GetFingerprint()), "fingerprint at index %d is not equal to corresponding return value", i)
		} else {
			// if current fingerprint is absent from storage, the corresponding index should be empty i.e. "nil"
			require.Nil(t, matchables[i], "resulting index %d should be of nil type", i)
		}
	}
}

func TestIndexInBounds(t *testing.T) {
	persister := testutil.NewUnscopedMockPersister()
	pollsToArchive := 100
	tracker := NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, pollsToArchive, persister).(*fileTracker)

	// no index exists. archiveIndex should be 0
	require.Equal(t, 0, tracker.archiveIndex)

	// run archiving. Each time, index should be in bound.
	for i := 0; i < 1099; i++ {
		require.Equalf(t, i%pollsToArchive, tracker.archiveIndex, "Index should %d, but was %d", i%pollsToArchive, tracker.archiveIndex)
		tracker.archive(&fileset.Fileset[*reader.Metadata]{})
		require.Truef(t, tracker.archiveIndex >= 0 && tracker.archiveIndex < pollsToArchive, "Index should be between 0 and %d, but was %d", pollsToArchive, tracker.archiveIndex)
	}
	oldIndex := tracker.archiveIndex

	// re-create archive
	tracker = NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, pollsToArchive, persister).(*fileTracker)

	// index should exist and new archiveIndex should be equal to oldIndex
	require.Equalf(t, oldIndex, tracker.archiveIndex, "New index should %d, but was %d", oldIndex, tracker.archiveIndex)

	// re-create archive, with reduced pollsToArchive
	pollsToArchive = 70
	tracker = NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, pollsToArchive, persister).(*fileTracker)

	// index should exist but it is out of bounds. So it should reset to 0
	require.Equalf(t, 0, tracker.archiveIndex, "Index should be reset to 0 but was %d", tracker.archiveIndex)
}

func TestArchiveRestoration(t *testing.T) {
	pollsToArchiveGrid := []int{10, 20, 50, 100, 200}
	for _, pollsToArchive := range pollsToArchiveGrid {
		for _, newPollsToArchive := range pollsToArchiveGrid {
			t.Run(fmt.Sprintf("%d-%d", pollsToArchive, newPollsToArchive), func(t *testing.T) {
				testArchiveRestoration(t, pollsToArchive, newPollsToArchive)
			})
		}
	}
}

func testArchiveRestoration(t *testing.T, pollsToArchive int, newPollsToArchive int) {
	//	test for various scenarios
	//		0.25 menas archive is 25% filled
	// 		1.25 means archive is 125% filled (i.e it was rolled over once)
	pctFilled := []float32{0.25, 0.5, 0.75, 1, 1.25, 1.50, 1.75, 2.00}
	for _, pct := range pctFilled {
		persister := testutil.NewUnscopedMockPersister()
		tracker := NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, pollsToArchive, persister).(*fileTracker)
		iterations := int(pct * float32(pollsToArchive))
		for i := 0; i < iterations; i++ {
			fileset := &fileset.Fileset[*reader.Metadata]{}
			fileset.Add(&reader.Metadata{
				// for the sake of this test case.
				// bigger the offset, more recent the element
				Offset: int64(i),
			})
			tracker.archive(fileset)
		}
		// make sure all keys are present in persister
		for i := 0; i < iterations; i++ {
			archiveIndex := i % pollsToArchive
			val, err := persister.Get(context.Background(), archiveKey(archiveIndex))
			require.NoError(t, err)
			require.NotNil(t, val)
		}
		// also, make sure we have not written "extra" stuff (for partially filled archive)
		count := 0
		for i := 0; i < pollsToArchive; i++ {
			val, err := persister.Get(context.Background(), archiveKey(i))
			require.NoError(t, err)
			if val != nil {
				count++
			}
		}
		require.Equal(t, min(iterations, pollsToArchive), count)
		tracker = NewFileTracker(context.Background(), componenttest.NewNopTelemetrySettings(), 0, newPollsToArchive, persister).(*fileTracker)
		if pollsToArchive > newPollsToArchive {
			// if archive has shrunk, new archive should contain most recent elements
			// start from most recent element
			startIdx := mod(tracker.archiveIndex-1, newPollsToArchive)
			mostRecentIteration := iterations - 1
			for i := 0; i < newPollsToArchive; i++ {
				val, err := tracker.readArchive(startIdx)
				require.NoError(t, err)
				if val.Len() > 0 {
					element, err := val.Pop()
					require.NoError(t, err)
					foundIteration := int(element.Offset)
					require.Equal(t, mostRecentIteration, foundIteration)
				}
				mostRecentIteration--
				startIdx--
			}

			// make sure we've removed all extra keys
			for i := newPollsToArchive; i < pollsToArchive; i++ {
				val, err := persister.Get(context.Background(), archiveKey(newPollsToArchive))
				require.NoError(t, err)
				require.Nil(t, val)
			}
		}
	}
}

func populatedPersisterData(persister operator.Persister, fps []*fingerprint.Fingerprint) []bool {
	md := make([]*reader.Metadata, 0)

	fpInStorage := make([]bool, len(fps))
	for i, fp := range fps {
		// 50-50 chance that a fingerprint exists in the storage
		if rand.Float32() < 0.5 {
			md = append(md, &reader.Metadata{Fingerprint: fp})
			fpInStorage[i] = true // mark the fingerprint at index i in storage
		}
	}
	// save half keys in knownFiles0 and other half in knownFiles1
	_ = checkpoint.SaveKey(context.Background(), persister, md[:len(md)/2], "knownFiles0")
	_ = checkpoint.SaveKey(context.Background(), persister, md[len(md)/2:], "knownFiles1")
	return fpInStorage
}
