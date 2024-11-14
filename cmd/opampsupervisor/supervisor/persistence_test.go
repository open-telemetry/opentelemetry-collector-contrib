// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateOrLoadPersistentState(t *testing.T) {
	t.Run("Creates a new state file if it does not exist", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")
		state, err := loadOrCreatePersistentState(f)
		require.NoError(t, err)

		// instance ID should be populated
		require.NotEqual(t, uuid.Nil, state.InstanceID)
		require.FileExists(t, f)
	})

	t.Run("loads state from file if it exists", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")

		err := os.WriteFile(f, []byte(`instance_id: "018feed6-905b-7aa6-ba37-b0eec565de03"`), 0600)
		require.NoError(t, err)

		state, err := loadOrCreatePersistentState(f)
		require.NoError(t, err)

		// instance ID should be populated with value from file
		require.Equal(t, uuid.MustParse("018feed6-905b-7aa6-ba37-b0eec565de03"), state.InstanceID)
		require.FileExists(t, f)
	})
}

func TestPersistentState_SetInstanceID(t *testing.T) {
	f := filepath.Join(t.TempDir(), "state.yaml")
	state, err := createNewPersistentState(f)
	require.NoError(t, err)

	// instance ID should be populated
	require.NotEqual(t, uuid.Nil, state.InstanceID)
	require.FileExists(t, f)

	newUUID := uuid.MustParse("018fee1f-871a-7d82-b22f-478085b3a1d6")
	err = state.SetInstanceID(newUUID)
	require.NoError(t, err)

	require.Equal(t, newUUID, state.InstanceID)

	// Test that loading the state after setting the instance ID has the new instance ID
	loadedState, err := loadPersistentState(f)
	require.NoError(t, err)

	require.Equal(t, newUUID, loadedState.InstanceID)
}
