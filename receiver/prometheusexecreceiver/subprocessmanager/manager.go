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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager/config"
)

// Process struct holds all the info needed to instantiate a subprocess
type Process struct {
	Command string
	Port    int
	Env     []config.EnvConfig
}

const (
	// HealthyProcessTime is the default time a process needs to stay alive to be considered healthy
	HealthyProcessTime time.Duration = 30 * time.Minute
	// HealthyCrashCount is the amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	HealthyCrashCount int = 3
	// delayMutiplier is the factor by which the delay scales
	delayMultiplier float64 = 2.0
	// initialDelay is the initial delay before a process is restarted
	initialDelay time.Duration = 1 * time.Second
)

// Run will start the process and keep track of running time
func (proc *Process) Run(logger *zap.Logger) (time.Duration, error) {

	var argsSlice []string

	// Parse the command line string into arguments
	args, err := shellquote.Split(proc.Command)
	if err != nil {
		return 0, fmt.Errorf("could not parse command error: %w", err)
	}
	// Separate the executable from the flags for the Command object
	if len(args) > 1 {
		argsSlice = args[1:]
	}

	// Create the command object and attach current os environment + environment variables defined by user
	childProcess := exec.Command(args[0], argsSlice...)
	childProcess.Env = append(os.Environ(), formatEnvSlice(&proc.Env)...)

	// Handle the subprocess standard and error outputs in goroutines
	stdoutReader, stdoutErr := childProcess.StdoutPipe()
	if stdoutErr != nil {
		return 0, fmt.Errorf("could not get the command's stdout pipe, err: %w", stdoutErr)
	}
	go proc.pipeSubprocessOutput(bufio.NewReader(stdoutReader), logger, true)

	stderrReader, stderrErr := childProcess.StderrPipe()
	if stderrErr != nil {
		return 0, fmt.Errorf("could not get the command's stderr pipe, err: %w", stderrErr)
	}
	go proc.pipeSubprocessOutput(bufio.NewReader(stderrReader), logger, false)

	// Start and stop timer (elapsed) right before and after executing the command
	start := time.Now()
	errProcess := childProcess.Start()
	if errProcess != nil {
		return 0, fmt.Errorf("process could not start: %w", errProcess)
	}

	errProcess = childProcess.Wait()
	elapsed := time.Since(start)
	if errProcess != nil {
		return elapsed, fmt.Errorf("process error: %w", errProcess)
	}

	return elapsed, nil
}

// Log every line of the subprocesse's output using zap, until pipe is closed (EOF)
func (proc *Process) pipeSubprocessOutput(reader *bufio.Reader, logger *zap.Logger, isStdout bool) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logger.Info("subprocess logging failed", zap.String("error", err.Error()))
			break
		}

		line = strings.TrimSpace(line)
		if line != "" {
			if isStdout {
				logger.Info("subprocess output line", zap.String("output", line))
			} else {
				logger.Error("subprocess output line", zap.String("output", line))
			}
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

// GetDelay will compute the delay for a given process according to its crash count and time alive using an exponential backoff algorithm
func GetDelay(elapsed time.Duration, crashCount int) time.Duration {
	// Return initialDelay if the process is healthy (lasted longer than health duration) or has less or equal than 3 crashes - it could be trying to find a port
	if elapsed > HealthyProcessTime || crashCount <= HealthyCrashCount {
		return initialDelay
	}

	// Return initialDelay times 2 to the power of crashCount-3 (to offset for the 3 allowed crashes) added to a random number
	return initialDelay * time.Duration(math.Pow(delayMultiplier, float64(crashCount-HealthyCrashCount)+rand.Float64()))
}
