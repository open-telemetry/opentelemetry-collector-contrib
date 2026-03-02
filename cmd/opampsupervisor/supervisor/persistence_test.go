// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCreateOrLoadPersistentState(t *testing.T) {
	t.Run("Creates a new state file if it does not exist", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")
		state, err := loadOrCreatePersistentState(f, zap.NewNop())
		require.NoError(t, err)

		// instance ID should be populated && remote config status should be nil
		require.NotEqual(t, uuid.Nil, state.InstanceID)
		require.Nil(t, state.LastRemoteConfigStatus)
		require.FileExists(t, f)
	})

	t.Run("loads state from file if it exists", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "state.yaml")

		lastRemoteConfigHash, err := hex.DecodeString("259ac3f596a87e6b8ca3b431908ac237ed8ae86128274da7911fcbedb93186a3")
		require.NoError(t, err)

		err = os.WriteFile(f, []byte(`
instance_id: "018feed6-905b-7aa6-ba37-b0eec565de03"
last_remote_config_status:
  status: 3
  last_remote_config_hash: 259ac3f596a87e6b8ca3b431908ac237ed8ae86128274da7911fcbedb93186a3
  error_message: "test error message"`), 0o600)

		require.NoError(t, err)

		state, err := loadOrCreatePersistentState(f, zap.NewNop())
		require.NoError(t, err)

		// instance ID should be populated with value from file
		require.Equal(t, uuid.MustParse("018feed6-905b-7aa6-ba37-b0eec565de03"), state.InstanceID)
		require.Equal(t, &protobufs.RemoteConfigStatus{
			Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
			LastRemoteConfigHash: lastRemoteConfigHash,
			ErrorMessage:         "test error message",
		}, state.GetLastRemoteConfigStatus())
		require.FileExists(t, f)
	})
}

func TestPersistentState_SetInstanceID(t *testing.T) {
	f := filepath.Join(t.TempDir(), "state.yaml")
	state, err := createNewPersistentState(f, zap.NewNop())
	require.NoError(t, err)

	// instance ID should be populated
	require.NotEqual(t, uuid.Nil, state.InstanceID)
	require.FileExists(t, f)

	newUUID := uuid.MustParse("018fee1f-871a-7d82-b22f-478085b3a1d6")
	err = state.SetInstanceID(newUUID)
	require.NoError(t, err)

	require.Equal(t, newUUID, state.InstanceID)

	// Test that loading the state after setting the instance ID has the new instance ID
	loadedState, err := loadPersistentState(f, zap.NewNop())
	require.NoError(t, err)

	require.Equal(t, newUUID, loadedState.InstanceID)
}

func TestPersistentState_SetLastRemoteConfigStatus(t *testing.T) {
	f := filepath.Join(t.TempDir(), "state.yaml")
	state, err := createNewPersistentState(f, zap.NewNop())
	require.NoError(t, err)

	require.Nil(t, state.LastRemoteConfigStatus)
	require.FileExists(t, f)

	lastRemoteConfigHash, err := hex.DecodeString("259ac3f596a87e6b8ca3b431908ac237ed8ae86128274da7911fcbedb93186a3")
	require.NoError(t, err)

	err = state.SetLastRemoteConfigStatus(&protobufs.RemoteConfigStatus{
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		LastRemoteConfigHash: lastRemoteConfigHash,
	})
	require.NoError(t, err)

	// Test that loading the state after setting the last remote config status has the new status
	loadedState, err := loadPersistentState(f, zap.NewNop())
	require.NoError(t, err)

	require.Equal(t, &protobufs.RemoteConfigStatus{
		Status:               protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		LastRemoteConfigHash: lastRemoteConfigHash,
	}, loadedState.GetLastRemoteConfigStatus())
	require.FileExists(t, f)
}
