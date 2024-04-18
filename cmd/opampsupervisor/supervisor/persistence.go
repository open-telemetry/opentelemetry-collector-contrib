package supervisor

import (
	"crypto/rand"
	"errors"
	"os"
	"time"

	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
)

type persistentState struct {
	InstanceID ulid.ULID `yaml:"instance_id"`

	configPath string `yaml:"-"`
}

func (p *persistentState) SetInstanceID(id ulid.ULID) {
	p.writeState()
}

func (p *persistentState) writeState() error {
	by, err := yaml.Marshal(p)
	if err != nil {
		return err
	}

	return os.WriteFile(p.configPath, by, 0600)
}

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
	id, err := generateNewULID()
	if err != nil {
		return nil, err
	}

	return &persistentState{
		InstanceID: id,
		configPath: file,
	}, nil
}

func generateNewULID() (ulid.ULID, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now()), entropy)
	if err != nil {
		return ulid.ULID{}, err
	}

	return id, nil
}
