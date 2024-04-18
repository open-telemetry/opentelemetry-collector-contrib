package opampextension

import (
	"testing"
	"time"
)

func setOrphanPollInterval(t *testing.T, newInterval time.Duration) {
	old := orphanPollInterval
	orphanPollInterval = newInterval
	t.Cleanup(func() {
		orphanPollInterval = old
	})
}
