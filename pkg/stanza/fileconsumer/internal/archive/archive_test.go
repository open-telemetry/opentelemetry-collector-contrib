// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package archive_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive_test"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/archive"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestArchiveNoRollover(t *testing.T) {
	persister := testutil.NewUnscopedMockPersister()
	a := archive.New(context.Background(), zap.L(), 3, persister)

	fp1 := fingerprint.New([]byte("fp1"))
	fp2 := fingerprint.New([]byte("fp2"))
	fp3 := fingerprint.New([]byte("fp3"))

	// Simulate three consecutive poll cycles
	a.WriteFiles(context.Background(), getFileset(fp1))
	a.WriteFiles(context.Background(), getFileset(fp2))
	a.WriteFiles(context.Background(), getFileset(fp3))

	// All three fingerprints should still be present in the archive
	fp3Modified := fingerprint.New([]byte("fp3...."))
	foundMetadata := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp3Modified})
	require.True(t, fp3.Equal(foundMetadata[0].GetFingerprint()), "Expected fp3 to match")

	fp2Modified := fingerprint.New([]byte("fp2...."))
	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp2Modified})
	require.True(t, fp2.Equal(foundMetadata[0].GetFingerprint()), "Expected fp2 to match")

	fp1Modified := fingerprint.New([]byte("fp1...."))
	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp1Modified})
	require.True(t, fp1.Equal(foundMetadata[0].GetFingerprint()), "Expected fp1 to match")
}

func TestArchiveRollOver(t *testing.T) {
	persister := testutil.NewUnscopedMockPersister()
	a := archive.New(context.Background(), zap.L(), 3, persister)

	fp1 := fingerprint.New([]byte("fp1"))
	fp2 := fingerprint.New([]byte("fp2"))
	fp3 := fingerprint.New([]byte("fp3"))
	fp4 := fingerprint.New([]byte("fp4"))

	// Simulate four consecutive poll cycles
	a.WriteFiles(context.Background(), getFileset(fp1))
	a.WriteFiles(context.Background(), getFileset(fp2))
	a.WriteFiles(context.Background(), getFileset(fp3))
	a.WriteFiles(context.Background(), getFileset(fp4)) // This should evice fp1

	// The archive should now contain fp2, fp3, and fp4
	fp4Modified := fingerprint.New([]byte("fp4...."))
	foundMetadata := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp4Modified})
	require.Len(t, foundMetadata, 1)
	require.True(t, fp4.Equal(foundMetadata[0].GetFingerprint()), "Expected fp4 to match")

	fp3Modified := fingerprint.New([]byte("fp3...."))
	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp3Modified})
	require.Len(t, foundMetadata, 1)
	require.True(t, fp3.Equal(foundMetadata[0].GetFingerprint()), "Expected fp3 to match")

	fp2Modified := fingerprint.New([]byte("fp2...."))
	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp2Modified})
	require.Len(t, foundMetadata, 1)
	require.True(t, fp2.Equal(foundMetadata[0].GetFingerprint()), "Expected fp2 to match")

	// fp1 should have been evicted and thus not retrievable
	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp1})
	require.Nil(t, foundMetadata[0], "Expected fp1 to be evicted from archive")
}

func TestNopArchive(t *testing.T) {
	a := archive.New(context.Background(), zap.L(), 3, nil)

	fp1 := fingerprint.New([]byte("fp1"))
	fp2 := fingerprint.New([]byte("fp2"))
	fp3 := fingerprint.New([]byte("fp3"))

	// Simulate three consecutive poll cycles
	a.WriteFiles(context.Background(), getFileset(fp1))
	a.WriteFiles(context.Background(), getFileset(fp2))
	a.WriteFiles(context.Background(), getFileset(fp3))

	// All three fingerprints should not be present in the archive
	foundMetadata := a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp3})
	require.Nil(t, foundMetadata[0], "fingerprint should not be in nopArchive")

	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp2})
	require.Nil(t, foundMetadata[0], "fingerprint should not be in nopArchive")

	foundMetadata = a.FindFiles(context.Background(), []*fingerprint.Fingerprint{fp1})
	require.Nil(t, foundMetadata[0], "fingerprint should not be in nopArchive")
}

func getFileset(fp *fingerprint.Fingerprint) *fileset.Fileset[*reader.Metadata] {
	set := fileset.New[*reader.Metadata](0)
	set.Add(&reader.Metadata{Fingerprint: fp})
	return set
}
