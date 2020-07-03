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
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager/config"
)

// Process struct holds all the info needed for subprocesses
type Process struct {
	Command    string
	Port       int
	Env        []config.EnvConfig
	CustomName string
}

const (
	// HealthyProcessTime is the default time a process needs to stay alive to be considered healthy
	HealthyProcessTime time.Duration = 30 * time.Minute
	healthyCrashCount  int           = 3               // Amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	delayMultiplier    float64       = 2.0             // The factor by which the delay scales (i.e. doubling every crash)
	baseDelay          time.Duration = 1 * time.Second // Base exponential backoff dela
)

// StartProcess will start the process and keep track of running time
func StartProcess(proc *Process) (time.Duration, error) {
	var (
		start     time.Time
		elapsed   time.Duration
		command   string
		argsSlice []string
	)

	// Iterate over the space-delimited args in the command, and set it to the correct variables, since the Command object needs the args separated
	for i, val := range strings.Split(proc.Command, " ") {
		if i == 0 {
			command = val
		} else {
			argsSlice = append(argsSlice, val)
		}
	}

	// Create the command object and attach current os environment + env variables defined by user
	childProcess := exec.Command(command, argsSlice...)
	childProcess.Env = os.Environ()
	childProcess.Env = append(childProcess.Env, formatEnvSlice(&proc.Env)...)
	attachChildOutputToParent(childProcess)

	// Start and stop timer right before and after executing the command
	start = time.Now()
	errProcess := childProcess.Run()
	elapsed = time.Since(start)

	if errProcess != nil {
		log.Printf("%v process error: %v", proc.CustomName, errProcess) // TODO: update with better logging
	}

	return elapsed, nil
}

// formatEnvSlice will loop over the key-value pairs and format the slice correctly for use by the Command object ("name=value")
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

// GetDelay will compute the exponential backoff for a given process according to its crash count and time alive
func GetDelay(elapsed time.Duration, crashCount int) time.Duration {
	// Return baseDelay if the process is healthy (lasted longer than health duration) or has less or equal than 3 crashes - it could be trying to find a port
	if elapsed > HealthyProcessTime || crashCount <= healthyCrashCount {
		return baseDelay
	}

	// Return baseDelay times 2 to the power of crashCount-3 (to offset for the 3 allowed crashes) added to a random number, all in time.Duration
	return baseDelay * time.Duration(math.Pow(delayMultiplier, float64(crashCount-healthyCrashCount)+rand.Float64()))
}

// Swap the child processe's Stdout and Stderr to parent processe's Stdout/Stderr
func attachChildOutputToParent(childProcess *exec.Cmd) {
	childProcess.Stdout = os.Stdout
	childProcess.Stderr = os.Stderr
}
