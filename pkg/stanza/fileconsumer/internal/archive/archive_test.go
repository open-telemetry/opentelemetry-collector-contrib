// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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

	archive := NewArchive(context.Background(), zap.L(), 100, persister)
	matchables := archive.FindFiles(fps)

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
	testArchive := NewArchive(context.Background(), zap.L(), 100, persister).(*archive)

	// no index exists. archiveIndex should be 0
	require.Equal(t, 0, testArchive.archiveIndex)

	// run archiving. Each time, index should be in bound.
	for i := 0; i < 1098; i++ {
		require.Equalf(t, i%pollsToArchive, testArchive.archiveIndex, "Index should %d, but was %d", i%pollsToArchive, testArchive.archiveIndex)
		testArchive.WriteFiles(&fileset.Fileset[*reader.Metadata]{})
		require.Truef(t, testArchive.archiveIndex >= 0 && testArchive.archiveIndex < pollsToArchive, "Index should be between 0 and %d, but was %d", pollsToArchive, testArchive.archiveIndex)
	}
	oldIndex := testArchive.archiveIndex

	// re-create archive
	testArchive = NewArchive(context.Background(), zap.L(), 100, persister).(*archive)

	// index should exist and new archiveIndex should be equal to oldIndex
	require.Equalf(t, oldIndex, testArchive.archiveIndex, "New index should %d, but was %d", oldIndex, testArchive.archiveIndex)

	// re-create archive, with reduced pollsToArchive
	pollsToArchive = 70

	testArchive = NewArchive(context.Background(), zap.L(), pollsToArchive, persister).(*archive)
	// index should exist but it is out of bounds. So it should reset to 0
	require.Equalf(t, 0, testArchive.archiveIndex, "Index should be reset to 0 but was %d", testArchive.archiveIndex)
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
