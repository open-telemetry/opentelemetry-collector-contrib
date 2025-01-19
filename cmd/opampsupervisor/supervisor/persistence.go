// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"errors"
	"os"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// persistentState represents persistent state for the supervisor
type persistentState struct {
	InstanceID uuid.UUID `yaml:"instance_id"`

	// Path to the config file that the state should be saved to.
	// This is not marshaled.
	configPath string `yaml:"-"`
}

func (p *persistentState) SetInstanceID(id uuid.UUID) error {
	p.InstanceID = id
	return p.writeState()
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
func loadOrCreatePersistentState(file string) (*persistentState, error) {
	state, err := loadPersistentState(file)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return createNewPersistentState(file)
	case err != nil:
		return nil, err
	default:
		return state, nil
	}
}

func loadPersistentState(file string) (*persistentState, error) {
	var state *persistentState

	by, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(by, &state); err != nil {
		return nil, err
	}

	state.configPath = file

	return state, nil
}

func createNewPersistentState(file string) (*persistentState, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	p := &persistentState{
		InstanceID: id,
		configPath: file,
	}

	return p, p.writeState()
}
