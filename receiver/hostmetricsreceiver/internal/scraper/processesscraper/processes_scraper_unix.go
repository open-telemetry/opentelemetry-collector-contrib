// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	countByStatus := map[string]int64{}
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
			countByStatus[metadata.AttributeStatus.Unknown]++
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

	countByStatus[metadata.AttributeStatus.Blocked] = int64(miscStat.ProcsBlocked)
	countByStatus[metadata.AttributeStatus.Running] = int64(miscStat.ProcsRunning)

	totalKnown := int64(0)
	for _, count := range countByStatus {
		totalKnown += count
	}
	if int64(miscStat.ProcsTotal) > totalKnown {
		countByStatus[metadata.AttributeStatus.Unknown] = int64(miscStat.ProcsTotal) - totalKnown
	}

	return processesMetadata{
		countByStatus:    countByStatus,
		processesCreated: procsCreated,
	}, nil
}

func toAttributeStatus(status []string) (string, bool) {
	if len(status) == 0 || len(status[0]) == 0 {
		return "", false
	}
	state, ok := charToState[status[0]]
	return state, ok
}

var charToState = map[string]string{
	process.Blocked:  metadata.AttributeStatus.Blocked,
	process.Daemon:   metadata.AttributeStatus.Daemon,
	process.Detached: metadata.AttributeStatus.Detached,
	process.Idle:     metadata.AttributeStatus.Idle,
	process.Lock:     metadata.AttributeStatus.Locked,
	process.Orphan:   metadata.AttributeStatus.Orphan,
	process.Running:  metadata.AttributeStatus.Running,
	process.Sleep:    metadata.AttributeStatus.Sleeping,
	process.Stop:     metadata.AttributeStatus.Stopped,
	process.System:   metadata.AttributeStatus.System,
	process.Wait:     metadata.AttributeStatus.Paging,
	process.Zombie:   metadata.AttributeStatus.Zombies,
}
