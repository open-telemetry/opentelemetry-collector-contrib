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
	// healthyCrashCount is the amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	healthyCrashCount int = 3
	// delayMutiplier is the factor by which the delay scales (default is doubling every crash)
	delayMultiplier float64 = 2.0
	// baseDelay is the base exponential backoff delay that every process waits for before restarting
	baseDelay time.Duration = 1 * time.Second
)

// StartProcess will start the process and keep track of running time
func StartProcess(proc *Process) (time.Duration, error) {
	var (
		start     time.Time
		elapsed   time.Duration
		command   string
		argsSlice []string
	)

	// Iterate over the space-delimited argumentss in the command, and set it to the correct variables, since the Command object needs the arguments and flags separated
	for _, val := range strings.Split(proc.Command, " ") {
		// This is to filter out empty strings in case the user put double spaces or other whitespace in the config by mistake
		if val == "" {
			continue
		}

		// The first word in the command is the binary to run, and the rest are flags
		if command == "" {
			command = val
		} else {
			argsSlice = append(argsSlice, val)
		}
	}

	// Create the command object and attach current os environment + environment variables defined by user
	childProcess := exec.Command(command, argsSlice...)
	childProcess.Env = os.Environ()
	childProcess.Env = append(childProcess.Env, formatEnvSlice(&proc.Env)...)

	// For now, simply attach the child processe's output to the parent's
	attachChildOutputToParent(childProcess)

	// Start and stop timer right before and after executing the command
	start = time.Now()
	errProcess := childProcess.Run()
	elapsed = time.Since(start)

	if errProcess != nil {
		return elapsed, fmt.Errorf("[%v] process error: %v", proc.CustomName, errProcess)
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

	// Return baseDelay times 2 to the power of crashCount-3 (to offset for the 3 allowed crashes) added to a random number
	return baseDelay * time.Duration(math.Pow(delayMultiplier, float64(crashCount-healthyCrashCount)+rand.Float64()))
}

// Swap the child processe's Stdout and Stderr to parent processe's Stdout/Stderr
func attachChildOutputToParent(childProcess *exec.Cmd) {
	childProcess.Stdout = os.Stdout
	childProcess.Stderr = os.Stderr
}
