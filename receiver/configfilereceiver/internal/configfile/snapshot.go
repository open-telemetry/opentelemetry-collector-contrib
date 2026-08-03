package configfile

import (
	"os"
)

// Snapshot is a parsed config file ready for OTLP export.
type Snapshot struct {
	File      string
	Format    string
	Checksum  string
	Keys      map[string]string
	KeysTotal int
	Event     string
}

// Event type for config snapshots.
const (
	EventInitial = "initial"
	EventChanged = "changed"
)

// BuildSnapshot parses path and returns a snapshot without comparing state.
func BuildSnapshot(path, format string, opts Options, event string) (*Snapshot, error) {
	resolved, keys, err := ParseFile(path, format, opts)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		File:      path,
		Format:    resolved,
		Checksum:  Checksum(keys),
		Keys:      keys,
		KeysTotal: len(keys),
		Event:     event,
	}, nil
}

// ProcessEntry evaluates one configured file against state. Returns snapshot when
// a log should be emitted, or nil when unchanged.
func ProcessEntry(entry FileEntry, st *State, opts Options, firstRun bool) (*Snapshot, bool, error) {
	if entry.Path == "" {
		return nil, false, nil
	}

	info, err := os.Stat(entry.Path)
	if err != nil {
		return nil, false, err
	}

	prev, hasPrev := st.Files[entry.Path]
	mtimeNS := info.ModTime().UnixNano()
	size := info.Size()

	if hasPrev && prev.MtimeNS == mtimeNS && prev.Size == size {
		return nil, false, nil
	}

	snap, err := BuildSnapshot(entry.Path, entry.Format, opts, EventChanged)
	if err != nil {
		return nil, false, err
	}
	if firstRun || !hasPrev {
		snap.Event = EventInitial
	}

	if hasPrev && snap.Checksum == prev.Checksum {
		st.Files[entry.Path] = FileState{
			MtimeNS:  mtimeNS,
			Size:     size,
			Checksum: prev.Checksum,
		}
		return nil, false, nil
	}

	st.Files[entry.Path] = FileState{
		MtimeNS:  mtimeNS,
		Size:     size,
		Checksum: snap.Checksum,
	}
	return snap, true, nil
}
