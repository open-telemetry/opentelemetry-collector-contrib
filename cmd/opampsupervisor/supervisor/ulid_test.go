package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestGenerateNewULID(t *testing.T) {
	// Test generating a new ULID twice returns 2 different results
	id1, err := generateNewULID()
	require.NoError(t, err)

	id2, err := generateNewULID()
	require.NoError(t, err)

	require.NotEqual(t, id1, id2)
}

func TestLoadSaveULIDRoundTrip(t *testing.T) {
	testULID := ulid.MustParse("01HVMZA0E0G1SFZEY16VCKMDDZ")

	tmpDir := t.TempDir()
	ulidFilePath := filepath.Join(tmpDir, "ulid.txt")

	err := saveULIDToFile(ulidFilePath, testULID)
	require.NoError(t, err)
	require.FileExists(t, ulidFilePath)

	loadedULID, err := loadULIDFromFile(ulidFilePath)
	require.NoError(t, err)

	require.Equal(t, testULID, loadedULID)
}

func TestLoadULID_FileDoesNotExist(t *testing.T) {
	_, err := loadULIDFromFile("./does_not_exist")
	require.ErrorIs(t, err, os.ErrNotExist)
}
