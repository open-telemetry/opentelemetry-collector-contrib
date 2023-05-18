// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin || freebsd || openbsd
// +build linux darwin freebsd openbsd

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

import (
	"runtime"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper/internal/metadata"
)

const enableProcessesCount = true
const enableProcessesCreated = runtime.GOOS == "openbsd" || runtime.GOOS == "linux"

func (s *scraper) getProcessesMetadata() (processesMetadata, error) {
	processes, err := s.getProcesses()
	if err != nil {
		return processesMetadata{}, err
	}

	countByStatus := map[metadata.AttributeStatus]int64{}
	for _, process := range processes {
		var status []string
		status, err = process.Status()
		if err != nil {
			// We expect an error in the case that a process has
			// been terminated as we run this code.
			continue
		}
		state, ok := toAttributeStatus(status)
		if !ok {
			countByStatus[metadata.AttributeStatusUnknown]++
			continue
		}
		countByStatus[state]++
	}

	// Processes are actively changing as we run this code, so this reason
	// the above loop will tend to underestimate process counts.
	// getMiscStats is a single read/syscall so it should be more accurate.
	miscStat, err := s.getMiscStats()
	if err != nil {
		return processesMetadata{}, err
	}

	var procsCreated *int64
	if enableProcessesCreated {
		v := int64(miscStat.ProcsCreated)
		procsCreated = &v
	}

	countByStatus[metadata.AttributeStatusBlocked] = int64(miscStat.ProcsBlocked)
	countByStatus[metadata.AttributeStatusRunning] = int64(miscStat.ProcsRunning)

	totalKnown := int64(0)
	for _, count := range countByStatus {
		totalKnown += count
	}
	if int64(miscStat.ProcsTotal) > totalKnown {
		countByStatus[metadata.AttributeStatusUnknown] = int64(miscStat.ProcsTotal) - totalKnown
	}

	return processesMetadata{
		countByStatus:    countByStatus,
		processesCreated: procsCreated,
	}, nil
}

func toAttributeStatus(status []string) (metadata.AttributeStatus, bool) {
	if len(status) == 0 || len(status[0]) == 0 {
		return metadata.AttributeStatus(0), false
	}
	state, ok := charToState[status[0]]
	return state, ok
}

var charToState = map[string]metadata.AttributeStatus{
	process.Blocked:  metadata.AttributeStatusBlocked,
	process.Daemon:   metadata.AttributeStatusDaemon,
	process.Detached: metadata.AttributeStatusDetached,
	process.Idle:     metadata.AttributeStatusIdle,
	process.Lock:     metadata.AttributeStatusLocked,
	process.Orphan:   metadata.AttributeStatusOrphan,
	process.Running:  metadata.AttributeStatusRunning,
	process.Sleep:    metadata.AttributeStatusSleeping,
	process.Stop:     metadata.AttributeStatusStopped,
	process.System:   metadata.AttributeStatusSystem,
	process.Wait:     metadata.AttributeStatusPaging,
	process.Zombie:   metadata.AttributeStatusZombies,
}
