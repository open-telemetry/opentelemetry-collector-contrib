// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive_test"

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestArchiveCRUD(t *testing.T) {
	// pollsToArchiveMatrix contains different polls_to_archive settings to test
	pollsToArchiveMatrix := []int{10, 20, 50, 100, 200}

	for _, p := range pollsToArchiveMatrix {
		t.Run(fmt.Sprintf("pollsToArchive:%d", p), func(t *testing.T) { testArchive(t, p) })
	}

	t.Run("nopArchive", func(t *testing.T) {
		testArchiveNop(t)
	})
}

func testArchive(t *testing.T, pollsToArchive int) {
	persister := testutil.NewUnscopedMockPersister()
	a := archive.New(context.Background(), zap.L(), pollsToArchive, persister)

	m := make(map[int]*fingerprint.Fingerprint)

	// rolledOverFps contains the fingerprints that were previously a part of archive, but are now rolled over i.e. removed.
	// archive should no longer contain such fingerprints
	rolledOverFps := make([]*fingerprint.Fingerprint, 0)

	for i := 0; i < 50; i++ {
		fp := fingerprint.New([]byte(createRandomString(100)))

		if oldFp, isRollover := m[i%pollsToArchive]; isRollover {
			// store the fp if we've already written to this index
			rolledOverFps = append(rolledOverFps, oldFp)
		}

		m[i%pollsToArchive] = fp

		set := fileset.New[*reader.Metadata](0)
		set.Add(&reader.Metadata{Fingerprint: fp})

		a.WriteFiles(context.Background(), set)
	}

	// sub-test 1: rolled over fingerprints should not be a part of archive
	for _, fp := range rolledOverFps {
		matchedData := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp})
		require.Nil(t, matchedData[0])
	}

	// sub-test 2: newer fingerprints should be part of archive
	for _, fp := range m {
		// FindFiles removes the data from persister.
		matchedData := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp})
		require.NotNil(t, matchedData[0])
		require.True(t, fp.Equal(matchedData[0].GetFingerprint()), "expected fingerprints to match")

		// archive should no longer contain data (as FindFiles removed the data)
		matchedData = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp})
		require.Nil(t, matchedData[0])
	}
}

func testArchiveNop(t *testing.T) {
	persister := testutil.NewUnscopedMockPersister()
	a := archive.New(context.Background(), zap.L(), -1, persister)

	m := make(map[int]*fingerprint.Fingerprint)

	for i := 0; i < 50; i++ {
		fp := fingerprint.New([]byte(createRandomString(100)))

		m[i] = fp

		set := fileset.New[*reader.Metadata](0)
		set.Add(&reader.Metadata{Fingerprint: fp})

		a.WriteFiles(context.Background(), set)
	}

	// fingerprints should not be part of archive, as it's nop
	for _, fp := range m {
		matchedData := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp})
		require.Nil(t, matchedData[0], "fingerprints shouldn't match for nopArchive")
	}
}

func createRandomString(length int) string {
	// some characters for the random generation
	const letterBytes = " ,.;:*-+/[]{}<>abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.IntN(len(letterBytes))]
	}

	return string(b)
}
