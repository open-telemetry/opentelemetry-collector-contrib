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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"go.uber.org/zap"
)

// Run will start the process and keep track of running time
func (proc *SubprocessConfig) Run(ctx context.Context, logger *zap.Logger) (time.Duration, error) {
	// Parse the command line string into arguments
	args, err := shellquote.Split(proc.Command)
	if err != nil {
		return 0, fmt.Errorf("could not parse command, error: %w", err)
	}

	// Create the command object and attach current os environment + environment variables defined by the user
	childProcess := exec.Command(args[0], args[1:]...) // #nosec
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
	processErrCh := make(chan error, 1)
	start := time.Now()

	errProcess := childProcess.Start()
	if errProcess != nil {
		return 0, fmt.Errorf("process could not start: %w", errProcess)
	}

	go func() {
		processErrCh <- childProcess.Wait()
	}()

	// Handle normal process exiting or parent logic triggering a shutdown
	select {
	case errProcess = <-processErrCh:
		elapsed := time.Since(start)

		if errProcess != nil {
			return elapsed, fmt.Errorf("%w", errProcess)
		}
		return elapsed, nil

	case <-ctx.Done():
		elapsed := time.Since(start)

		err := childProcess.Process.Kill()
		if err != nil {
			return elapsed, fmt.Errorf("couldn't kill subprocess: %w", errProcess)
		}
		return elapsed, nil
	}
}

// Log every line of the subprocesse's output using zap, until pipe is closed (EOF)
func (proc *SubprocessConfig) pipeSubprocessOutput(reader *bufio.Reader, logger *zap.Logger, isStdout bool) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			logger.Info("subprocess logging failed", zap.String("error", err.Error()))
			break
		}

		line = strings.TrimSpace(line)
		if line != "" && line != "\n" {
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
func formatEnvSlice(envs *[]EnvConfig) []string {
	if len(*envs) == 0 {
		return nil
	}

	envSlice := make([]string, len(*envs))
	for i, env := range *envs {
		envSlice[i] = fmt.Sprintf("%v=%v", env.Name, env.Value)
	}

	return envSlice
}
