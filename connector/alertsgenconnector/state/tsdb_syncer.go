
package state

import (
	"time"
)

type TSDBSyncer struct {
	QueryURL        string
	InstanceID      string
	QueryInterval   time.Duration
	TakeoverTimeout time.Duration
}

type ActiveEntry struct {
	Active      bool
	Labels      map[string]string
	ForDuration time.Duration
}

func (s *TSDBSyncer) QueryActive(rule string) (map[uint64]ActiveEntry, error) {
	// Best-effort stub: integration with Prometheus-compatible remote_read API can be added.
	return map[uint64]ActiveEntry{}, nil
}
