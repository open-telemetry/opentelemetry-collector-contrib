// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package subprocessmanager

import (
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

// Process struct holds all the info needed for subprocesses
type Process struct {
	Command    string
	Port       int
	Env        []string
	CustomName string
}

const (
	healthyProcessTime time.Duration = 30 * time.Minute // Default time for a process to stay alive and be considered healthy
	healthyCrashCount  int           = 3                // Amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	delayMultiplier    float64       = 2.0              // The factor by which the delay scales (i.e. doubling every crash)
	baseDelay          time.Duration = 1 * time.Second  // Base exponential backoff delay
)

// StartProcess will put a process in an infinite starting loop (if the process crashes there is a delay before it starts again computed by the exponentional backoff algorithm)
func StartProcess(proc *Process) {
	var (
		start      time.Time
		elapsed    time.Duration
		crashCount int
	)

	for true {
		// Create the command object and attach current os environment + env variables configured by user
		childProcess := exec.Command(proc.Command)
		childProcess.Env = os.Environ()
		childProcess.Env = append(childProcess.Env, proc.Env...)
		attachChildOutputToParent(childProcess)

		// Start and stop timer right before and after executing command
		start = time.Now()
		errProcess := childProcess.Run()
		elapsed = time.Since(start)

		if errProcess != nil {
			log.Printf("%v", errProcess) // TODO: update with better logging
		}

		// Reset crash count to 1 if the process seems to be healthy now
		if elapsed > healthyProcessTime {
			crashCount = 1
		} else {
			crashCount++
		}
		// Sleep this goroutine for a certain amount of time, computed by exponential backoff
		time.Sleep(getDelay(elapsed, crashCount))
	}
}

func getDelay(elapsed time.Duration, crashCount int) time.Duration {
	// Return baseDelay if the process is healthy (lasted longer than health duration) or has less or equal than 3 crashes - it could be trying to find a port
	if elapsed > healthyProcessTime || crashCount <= healthyCrashCount {
		return baseDelay
	}

	// Return baseDelay times 2 to the power of crashCount-3 (to offset for the 3 allowed crashes) added to a random number, all in time.Duration
	return baseDelay * time.Duration(math.Pow(delayMultiplier, float64(crashCount-healthyCrashCount)+rand.Float64()))
}

// Swap the child processes' Stdout and Stderr to parent processe's Stdout/Stderr for now
func attachChildOutputToParent(childProcess *exec.Cmd) {
	childProcess.Stdout = os.Stdout
	childProcess.Stderr = os.Stderr
}
