package configfile

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultStatePath = "/opt/asama.ai/cache/configfile-state.json"

// FileState tracks last observed metadata and checksum for a config path.
type FileState struct {
	MtimeNS  int64  `json:"mtime_ns"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

// State maps absolute file paths to persisted metadata.
type State struct {
	Files map[string]FileState `json:"files"`
}

// LoadState reads state from disk or returns an empty state if missing.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Files: make(map[string]FileState)}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Files == nil {
		st.Files = make(map[string]FileState)
	}
	return &st, nil
}

// SaveState writes state atomically.
func SaveState(path string, st *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
