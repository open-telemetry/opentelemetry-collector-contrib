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
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"go.uber.org/zap"

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

// Run will start the process and keep track of running time
func (proc *Process) Run(logger *zap.Logger) (time.Duration, error) {
	var (
		start     time.Time
		elapsed   time.Duration
		argsSlice []string
	)

	// Parse the command line string into arguments
	args, err := shellquote.Split(proc.Command)
	if err != nil {
		return elapsed, fmt.Errorf("[%v] could not parse command error: %v", proc.CustomName, err)
	}
	// Separate the executable from the flags for the Command object
	if len(args) > 1 {
		argsSlice = args[1:]
	}

	// Create the command object and attach current os environment + environment variables defined by user
	childProcess := exec.Command(args[0], argsSlice...)
	childProcess.Env = os.Environ()
	childProcess.Env = append(childProcess.Env, formatEnvSlice(&proc.Env)...)

	// Handle the subprocess output in a goroutine, and pipe the stderr/stdout into one stream
	cmdReader, err := childProcess.StdoutPipe()
	if err != nil {
		return elapsed, fmt.Errorf("[%v] could not get the command's output pipe, err: %v", proc.CustomName, err)
	}
	childProcess.Stderr = childProcess.Stdout
	go proc.handleSubprocessOutput(bufio.NewReader(cmdReader), logger)

	// Start and stop timer (elapsed) right before and after executing the command
	start = time.Now()
	errProcess := childProcess.Start()
	if errProcess != nil {
		return elapsed, fmt.Errorf("[%v] process could not start: %v", proc.CustomName, errProcess)
	}

	errProcess = childProcess.Wait()
	elapsed = time.Since(start)
	if errProcess != nil {
		return elapsed, fmt.Errorf("[%v] process error: %v", proc.CustomName, errProcess)
	}

	return elapsed, nil
}

// Log every line of the subprocesse's output using zap
func (proc *Process) handleSubprocessOutput(reader *bufio.Reader, logger *zap.Logger) {
	var (
		line string
		err  error
	)

	// Infinite reading loop until EOF (pipe is closed)
	for {
		line, err = reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logger.Info("subprocess logging failed", zap.String("subprocess name", proc.CustomName), zap.String("error", err.Error()))
			break
		}

		line = strings.TrimSpace(line)
		if line != "" {
			logger.Info("subprocess output line", zap.String("subprocess name", proc.CustomName), zap.String("output", line))
		}

		// Leave this function when error is EOF (stderr/stdout pipe was closed)
		if err == io.EOF {
			break
		}
	}
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
