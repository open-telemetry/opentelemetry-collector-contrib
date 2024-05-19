// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestCreateOrLoadPersistentState(t *testing.T) {
	t.Run("Creates a new state file if it does not exist", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")
		state, err := loadOrCreatePersistentState(f)
		require.NoError(t, err)

		// instance ID should be populated
		require.NotEqual(t, ulid.ULID{}, state.InstanceID)
		require.FileExists(t, f)
	})

	t.Run("loads state from file if it exists", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")

		err := os.WriteFile(f, []byte(`instance_id: "01HW3GS9NWD840C5C2BZS3KYPW"`), 0600)
		require.NoError(t, err)

		state, err := loadOrCreatePersistentState(f)
		require.NoError(t, err)

		// instance ID should be populated with value from file
		require.Equal(t, ulid.MustParse("01HW3GS9NWD840C5C2BZS3KYPW"), state.InstanceID)
		require.FileExists(t, f)
	})

}

func TestPersistentState_SetInstanceID(t *testing.T) {
	f := filepath.Join(t.TempDir(), "state.yaml")
	state, err := createNewPersistentState(f)
	require.NoError(t, err)

	// instance ID should be populated
	require.NotEqual(t, ulid.ULID{}, state.InstanceID)
	require.FileExists(t, f)

	newULID := ulid.MustParse("01HW3GS9NWD840C5C2BZS3KYPW")
	err = state.SetInstanceID(newULID)
	require.NoError(t, err)

	require.Equal(t, newULID, state.InstanceID)

	// Test that loading the state after setting the instance ID has the new instance ID
	loadedState, err := loadPersistentState(f)
	require.NoError(t, err)

	require.Equal(t, newULID, loadedState.InstanceID)
}

func TestGenerateNewULID(t *testing.T) {
	// Test generating a new ULID twice returns 2 different results
	id1, err := generateNewULID()
	require.NoError(t, err)

	id2, err := generateNewULID()
	require.NoError(t, err)

	require.NotEqual(t, id1, id2)
}
