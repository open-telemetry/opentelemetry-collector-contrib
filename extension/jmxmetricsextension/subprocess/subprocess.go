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

package subprocess

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// exported to be used by jmx metrics extension
type Config struct {
	ExecutablePath       string            `mapstructure:"executable_path"`
	Args                 []string          `mapstructure:"args"`
	EnvironmentVariables map[string]string `mapstructure:"environment_variables"`
	StdInContents        string            `mapstructure:"stdin_contents"`
	RestartOnError       bool              `mapstructure:"restart_on_error"`
}

// exported to be used by jmx metrics extension
type Subprocess struct {
	Stdout         chan string
	cancel         context.CancelFunc
	config         *Config
	envVars        []string
	logger         *zap.Logger
	pid            *pid
	shutdownSignal chan struct{}
	// configurable for testing purposes
	sendToStdIn func(string, io.Writer) error
}

type pid struct {
	pid     int
	pidLock sync.Mutex
}

func (p *pid) setPid(pid int) {
	p.pidLock.Lock()
	defer p.pidLock.Unlock()
	p.pid = pid
}

func (p *pid) getPid() int {
	p.pidLock.Lock()
	defer p.pidLock.Unlock()
	return p.pid
}

func (subprocess *Subprocess) Pid() int {
	return subprocess.pid.getPid()
}

// exported to be used by jmx metrics extension
func NewSubprocess(conf *Config, logger *zap.Logger) *Subprocess {
	return &Subprocess{
		Stdout:         make(chan string),
		pid:            &pid{pid: -1, pidLock: sync.Mutex{}},
		config:         conf,
		logger:         logger,
		shutdownSignal: make(chan struct{}),
		sendToStdIn:    sendToStdIn,
	}
}

// A global var that is available only for testing
var restartDelay = 5 * time.Second

const (
	Starting     = "Starting"
	Running      = "Running"
	ShuttingDown = "ShuttingDown"
	Stopped      = "Stopped"
	Restarting   = "Restarting"
	Errored      = "Errored"
)

func (subprocess *Subprocess) Start(ctx context.Context) error {
	var cancelCtx context.Context
	cancelCtx, subprocess.cancel = context.WithCancel(ctx)

	for k, v := range subprocess.config.EnvironmentVariables {
		joined := fmt.Sprintf("%v=%v", k, v)
		subprocess.envVars = append(subprocess.envVars, joined)
	}

	go func() {
		subprocess.run(cancelCtx) // will block for lifetime of process
		close(subprocess.shutdownSignal)
	}()
	return nil
}

// Shutdown is invoked during service shutdown.
func (subprocess *Subprocess) Shutdown(ctx context.Context) error {
	subprocess.cancel()
	t := time.NewTimer(5 * time.Second)

	// Wait for the subprocess to exit or the timeout period to elapse
	select {
	case <-ctx.Done():
	case <-subprocess.shutdownSignal:
	case <-t.C:
	}

	return nil
}

// Core event loop
func (subprocess *Subprocess) run(ctx context.Context) {
	var cmd *exec.Cmd
	var err error
	var stdin io.WriteCloser
	var stdout io.ReadCloser

	// writer is signalWhenProcessReturned() and closer is this loop, so we need synchronization
	processReturned := make(chan error)
	processReturnedOpen := &atomic.Value{}
	processReturnedOpen.Store(true)
	processReturnedLock := &sync.Mutex{}

	state := Starting
	for {
		subprocess.logger.Debug("subprocess changed state", zap.String("state", state))

		switch state {
		case Starting:
			cmd, stdin, stdout = createCommand(
				subprocess.config.ExecutablePath,
				subprocess.config.Args,
				subprocess.envVars,
			)

			go collectStdout(stdout, subprocess.Stdout, subprocess.logger)

			subprocess.logger.Debug("starting subprocess", zap.String("command", cmd.String()))
			err = cmd.Start()
			if err != nil {
				state = Errored
				continue
			}
			subprocess.pid.setPid(cmd.Process.Pid)

			go signalWhenProcessReturned(cmd, processReturned, processReturnedLock, processReturnedOpen)

			state = Running
		case Running:
			err = subprocess.sendToStdIn(subprocess.config.StdInContents, stdin)
			stdin.Close()
			if err != nil {
				state = Errored
				continue
			}

			select {
			case err = <-processReturned:
				if err != nil && ctx.Err() == nil {
					err = fmt.Errorf("unexpected shutdown: %w", err)
					// We aren't supposed to shutdown yet so this is an error state.
					state = Errored
					continue
				}
				// We must close this channel or can wait indefinitely at ShuttingDown
				processReturnedLock.Lock()
				close(processReturned)
				processReturnedOpen.Store(false)
				processReturnedLock.Unlock()
				state = ShuttingDown
			case <-ctx.Done(): // context-based cancel.
				state = ShuttingDown
			}
		case Errored:
			subprocess.logger.Error("subprocess died", zap.Error(err))
			if subprocess.config.RestartOnError {
				subprocess.pid.setPid(-1)
				state = Restarting
			} else {
				// We must close this channel or can wait indefinitely at ShuttingDown
				processReturnedLock.Lock()
				close(processReturned)
				processReturnedOpen.Store(false)
				processReturnedLock.Unlock()
				state = ShuttingDown
			}
		case ShuttingDown:
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGTERM)
			}
			<-processReturned
			stdout.Close()
			subprocess.pid.setPid(-1)
			state = Stopped
		case Restarting:
			stdout.Close()
			stdin.Close()
			time.Sleep(restartDelay)
			state = Starting
		case Stopped:
			return
		}
	}
}

func signalWhenProcessReturned(cmd *exec.Cmd, processReturned chan<- error, lock *sync.Mutex, open *atomic.Value) {
	err := cmd.Wait()
	// we cannot close here as we may need to restart and channel persists in loop.
	// here we ensure we only write to open channel
	lock.Lock()
	defer lock.Unlock()
	if open.Load().(bool) {
		processReturned <- err
	}
}

func collectStdout(stdout io.Reader, stdoutChan chan<- string, logger *zap.Logger) {
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		text := scanner.Text()
		if text != "" {
			stdoutChan <- text
			logger.Debug(text)
		}
	}
	// Returns when stdout is closed when the process ends
}

func sendToStdIn(contents string, writer io.Writer) error {
	if contents == "" {
		return nil
	}

	_, err := writer.Write([]byte(contents))
	return err
}

func createCommand(execPath string, args, envVars []string) (*exec.Cmd, io.WriteCloser, io.ReadCloser) {
	cmd := exec.Command(execPath, args...)

	var env []string
	env = append(env, os.Environ()...)
	cmd.Env = append(env, envVars...)

	inReader, inWriter, err := os.Pipe()
	if err != nil {
		panic("Input pipe could not be created for subprocess")
	}

	cmd.Stdin = inReader

	outReader, outWriter, err := os.Pipe()
	if err != nil {
		panic("Output pipe could not be created for subprocess")
	}
	cmd.Stdout = outWriter
	cmd.Stderr = outWriter

	applyOSSpecificCmdModifications(cmd)

	return cmd, inWriter, outReader
}
