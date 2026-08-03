package configfile

import (
	"log/slog"
	"os"
)

const defaultPollIntervalSeconds = 60

// Poller watches configured files and yields snapshots when content changes.
type Poller struct {
	cfg   PollerConfig
	state *State
}

// NewPoller builds a poller from settings.
func NewPoller(cfg PollerConfig) *Poller {
	if cfg.PollIntervalSecond <= 0 {
		cfg.PollIntervalSecond = defaultPollIntervalSeconds
	}
	if cfg.StatePath == "" {
		cfg.StatePath = DefaultStatePath
	}
	return &Poller{cfg: cfg}
}

// LoadState reads persisted mtime/checksum state from disk.
func (p *Poller) LoadState() error {
	st, err := LoadState(p.cfg.StatePath)
	if err != nil {
		return err
	}
	p.state = st
	return nil
}

// SaveState persists current state.
func (p *Poller) SaveState() error {
	return SaveState(p.cfg.StatePath, p.state)
}

// Poll processes all configured files and returns snapshots that should be emitted.
func (p *Poller) Poll(firstRun bool) []*Snapshot {
	opts := p.cfg.options()
	var snaps []*Snapshot

	for _, entry := range p.cfg.Files {
		if entry.Path == "" {
			continue
		}
		if _, err := os.Stat(entry.Path); err != nil {
			if os.IsNotExist(err) {
				slog.Debug("configfile: file missing", "path", entry.Path)
				continue
			}
			slog.Warn("configfile: stat failed", "path", entry.Path, "err", err)
			continue
		}

		snap, emit, err := ProcessEntry(entry, p.state, opts, firstRun)
		if err != nil {
			slog.Warn("configfile: process failed", "path", entry.Path, "err", err)
			continue
		}
		if emit && snap != nil {
			snaps = append(snaps, snap)
			slog.Info("configfile: snapshot",
				"path", snap.File,
				"event", snap.Event,
				"keys", snap.KeysTotal,
				"checksum", snap.Checksum,
			)
		}
	}
	return snaps
}
