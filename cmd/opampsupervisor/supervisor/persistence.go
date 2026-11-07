// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// persistentState represents persistent state for the supervisor
type persistentState struct {
	// InstanceID must be a valid UUID string. If it is not, a new UUIDv7 will be generated automatically.
	InstanceID             uuid.UUID           `yaml:"instance_id"`
	LastRemoteConfigStatus *RemoteConfigStatus `yaml:"last_remote_config_status"`

	// Path to the config file that the state should be saved to.
	// This is not marshaled.
	configPath string      `yaml:"-"`
	logger     *zap.Logger `yaml:"-"`
}

// RemoteConfigStatus is a custom struct that is used to marshal/unmarshal the remote config status.
// LastRemoteConfigHash is a hex encoded string of the last remote config hash for human readability.
type RemoteConfigStatus struct {
	// Status is stored cleanly as a plain string inside the YAML file ("applied", "failed", etc.)
	Status string `yaml:"status"`
	// LastRemoteConfigHash is a hex encoded string of the last remote config hash for human readability.
	LastRemoteConfigHash string `yaml:"last_remote_config_hash"`
	// ErrorMessage is the error message of the last remote config.
	ErrorMessage string `yaml:"error_message"`
}

func (p *persistentState) SetInstanceID(id uuid.UUID) error {
	p.InstanceID = id
	return p.writeState()
}

func remoteConfigStatusToString(status protobufs.RemoteConfigStatuses) string {
	switch status {
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED:
		return "applied"
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING:
		return "applying"
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED:
		return "failed"
	case protobufs.RemoteConfigStatuses_RemoteConfigStatuses_UNSET:
		return "unset"
	default:
		return "unset"
	}
}

func remoteConfigStatusFromString(status string) protobufs.RemoteConfigStatuses {
	switch strings.ToLower(status) {
	case "applied", "1":
		return protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
	case "applying", "2":
		return protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING
	case "failed", "3":
		return protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED
	case "unset", "0":
		return protobufs.RemoteConfigStatuses_RemoteConfigStatuses_UNSET
	default:
		return protobufs.RemoteConfigStatuses_RemoteConfigStatuses_UNSET
	}
}

func (p *persistentState) SetLastRemoteConfigStatus(
	status *protobufs.RemoteConfigStatus,
) error {
	p.LastRemoteConfigStatus = &RemoteConfigStatus{
		Status:               remoteConfigStatusToString(status.Status),
		LastRemoteConfigHash: hex.EncodeToString(status.LastRemoteConfigHash),
		ErrorMessage:         status.ErrorMessage,
	}
	return p.writeState()
}

func (p *persistentState) GetLastRemoteConfigStatus() *protobufs.RemoteConfigStatus {
	if p.LastRemoteConfigStatus == nil {
		return nil
	}
	lastRemoteConfigHash, err := hex.DecodeString(p.LastRemoteConfigStatus.LastRemoteConfigHash)
	if err != nil {
		p.logger.Error("Failed to decode last remote config hash, returning empty status", zap.Error(err))
		return nil
	}

	// Stably parse the plain text YAML string back to the native protobuf integer enum
	protobufStatus := remoteConfigStatusFromString(
		p.LastRemoteConfigStatus.Status,
	)

	return &protobufs.RemoteConfigStatus{
		Status:               protobufStatus,
		LastRemoteConfigHash: lastRemoteConfigHash,
		ErrorMessage:         p.LastRemoteConfigStatus.ErrorMessage,
	}
}

func (p *persistentState) writeState() error {
	by, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

	return os.WriteFile(p.configPath, by, 0o600)
}

// loadOrCreatePersistentState attempts to load the persistent state from disk. If it doesn't
// exist, a new persistent state file is created.
// instanceID must be a valid UUID string, or an empty string to generate a new UUIDv7 automatically.
func loadOrCreatePersistentState(file, instanceID string, logger *zap.Logger) (*persistentState, error) {
	state, err := loadPersistentState(file, logger)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return createNewPersistentState(file, instanceID, logger)
	case err != nil:
		return nil, err
	default:
		return state, nil
	}
}

func loadPersistentState(file string, logger *zap.Logger) (*persistentState, error) {
	var state *persistentState

	by, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(by, &state); err != nil {
		return nil, err
	}

	state.configPath = file
	state.logger = logger

	return state, nil
}

func createNewPersistentState(file, instanceID string, logger *zap.Logger) (*persistentState, error) {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		if instanceID != "" {
			logger.Warn("Failed to parse instance_id, generating one automatically", zap.Error(err))
		}
		id, err = uuid.NewV7()
		if err != nil {
			return nil, err
		}
	}

	p := &persistentState{
		InstanceID: id,
		configPath: file,
		logger:     logger,
	}

	err = p.writeState()
	return p, err
}
