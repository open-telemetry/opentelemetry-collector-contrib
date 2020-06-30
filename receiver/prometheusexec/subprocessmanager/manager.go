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
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager/config"
	"go.opentelemetry.io/collector/component"
)

// Process struct holds all the info needed for subprocesses
type Process struct {
	Command    string
	Port       int
	Env        []config.EnvConfig
	CustomName string

	// Receiver data
	Receiver component.MetricsReceiver
	Context  context.Context
	Host     component.Host
}

const (
	healthyProcessTime time.Duration = 30 * time.Minute // Default time for a process to stay alive and be considered healthy
	healthyCrashCount  int           = 3                // Amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	delayMultiplier    float64       = 2.0              // The factor by which the delay scales (i.e. doubling every crash)
	baseDelay          time.Duration = 1 * time.Second  // Base exponential backoff delay
)

// StartProcess will put a process in an infinite starting loop (if the process crashes there is a delay before it starts again computed by the exponentional backoff algorithm)
func StartProcess(proc *Process) error {
	var (
		start        time.Time
		elapsed      time.Duration
		crashCount   int
		originalPort int = proc.Port
		newPort      int
	)

	for true {
		if originalPort == 0 {
			newPort = generateRandomPort(newPort)
		}

		// Create the command object and attach current os environment + env variables configured by user
		childProcess := exec.Command(proc.Command)
		childProcess.Env = os.Environ()
		childProcess.Env = append(childProcess.Env, formatEnvSlice(&proc.Env)...)
		attachChildOutputToParent(childProcess)

		// Start and stop timer right before and after executing command
		start = time.Now()
		errProcess := childProcess.Run()
		elapsed = time.Since(start)

		if errProcess != nil {
			log.Printf("%v process error: %v", proc.CustomName, errProcess) // TODO: update with better logging
		}

		// Reset crash count to 1 if the process seems to be healthy now, else increase crashCount
		if elapsed > healthyProcessTime {
			crashCount = 1
		} else {
			crashCount++

			// Stop the associated receiver until process starts back up again if process is unhealthy, keep alive if process is deemed healthy to not lose data
			if crashCount > healthyCrashCount {
				err := proc.Receiver.Shutdown(proc.Context)
				if err != nil {
					return fmt.Errorf("could not stop receiver associated to %v process, killing this single process(%v)", proc.CustomName, proc.CustomName)
				}
			}
		}
		// Sleep this goroutine for a certain amount of time, computed by exponential backoff
		time.Sleep(getDelay(elapsed, crashCount))

		// Stop the associated receiver if process is unhealthy, keep alive if process is deemed healthy to not lose data
		if crashCount > healthyCrashCount {
			err := proc.Receiver.Start(proc.Context, proc.Host)
			if err != nil {
				return fmt.Errorf("could not restart receiver associated to %v process, killing this single process (%v)", proc.CustomName, proc.CustomName)
			}
		}
	}

	return nil
}

func generateRandomPort(lastPort int) int {
	for true {
		random := rand.New(rand.NewSource(0)).Seed(time.Now().UnixNano())
		fmt.Println(random.Intn(100))
	}
}

func formatEnvSlice(envs *[]config.EnvConfig) []string {
	if len(*envs) == 0 {
		return nil
	}

	envSlice := make([]string, len(*envs))
	for i, env := range *envs {
		envSlice[i] = fmt.Sprintf("%v=%v", env.Name, env.Value)
	}

	return envSlice
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
