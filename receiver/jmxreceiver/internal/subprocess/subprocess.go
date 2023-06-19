// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subprocess // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"

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

const (
	defaultRestartDelay    = 5 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	noPid                  = -1
)

// Config exported to be used by jmx metric receiver.
type Config struct {
	ExecutablePath       string            `mapstructure:"executable_path"`
	Args                 []string          `mapstructure:"args"`
	EnvironmentVariables map[string]string `mapstructure:"environment_variables"`
	StdInContents        string            `mapstructure:"stdin_contents"`
	RestartOnError       bool              `mapstructure:"restart_on_error"`
	RestartDelay         *time.Duration    `mapstructure:"restart_delay"`
	ShutdownTimeout      *time.Duration    `mapstructure:"shutdown_timeout"`
}

// Subprocess exported to be used by jmx metric receiver.
type Subprocess struct {
	Stdout         chan string
	cancel         context.CancelFunc
	config         *Config
	envVars        []string
	logger         *zap.Logger
	pid            pid
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
	pid := subprocess.pid.getPid()
	if pid == 0 {
		return noPid
	}
	return pid
}

// NewSubprocess exported to be used by jmx metric receiver.
func NewSubprocess(conf *Config, logger *zap.Logger) *Subprocess {
	if conf.RestartDelay == nil {
		restartDelay := defaultRestartDelay
		conf.RestartDelay = &restartDelay
	}
	if conf.ShutdownTimeout == nil {
		shutdownTimeout := defaultShutdownTimeout
		conf.ShutdownTimeout = &shutdownTimeout
	}

	return &Subprocess{
		Stdout:         make(chan string),
		pid:            pid{pid: noPid, pidLock: sync.Mutex{}},
		config:         conf,
		logger:         logger,
		shutdownSignal: make(chan struct{}),
		sendToStdIn:    sendToStdIn,
	}
}

const (
	starting     = "Starting"
	running      = "Running"
	shuttingDown = "ShuttingDown"
	stopped      = "Stopped"
	restarting   = "Restarting"
	errored      = "Errored"
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
	if subprocess.cancel == nil {
		return fmt.Errorf("no subprocess.cancel().  Has it been started properly?")
	}

	timeout := defaultShutdownTimeout
	if subprocess.config.ShutdownTimeout != nil {
		timeout = *subprocess.config.ShutdownTimeout
	}
	t := time.NewTimer(timeout)

	subprocess.cancel()

	// Wait for the subprocess to exit or the timeout period to elapse
	select {
	case <-ctx.Done():
	case <-subprocess.shutdownSignal:
	case <-t.C:
		subprocess.logger.Warn("subprocess hasn't returned within shutdown timeout. May be zombied.",
			zap.String("timeout", fmt.Sprintf("%v", timeout)))
	}

	return nil
}

// A synchronization helper to ensure that signalWhenProcessReturned
// doesn't write to a closed channel
type processReturned struct {
	ReturnedChan chan error
	isOpen       *atomic.Bool
	lock         *sync.Mutex
}

func newProcessReturned() *processReturned {
	isOpen := &atomic.Bool{}
	isOpen.Store(true)
	pr := processReturned{
		ReturnedChan: make(chan error),
		isOpen:       isOpen,
		lock:         &sync.Mutex{},
	}
	return &pr
}

func (pr *processReturned) signal(err error) {
	pr.lock.Lock()
	defer pr.lock.Unlock()
	if pr.isOpen.Load() {
		pr.ReturnedChan <- err
	}
}

func (pr *processReturned) close() {
	pr.lock.Lock()
	defer pr.lock.Unlock()
	if pr.isOpen.Load() {
		close(pr.ReturnedChan)
		pr.isOpen.Store(false)
	}
}

// Core event loop
func (subprocess *Subprocess) run(ctx context.Context) {
	var cmd *exec.Cmd
	var err error
	var stdin io.WriteCloser
	var stdout io.ReadCloser

	// writer is signalWhenProcessReturned() and closer is this loop, so we need synchronization
	processReturned := newProcessReturned()

	state := starting
	for {
		subprocess.logger.Debug("subprocess changed state", zap.String("state", state))

		switch state {
		case starting:
			cmd, stdin, stdout = createCommand(
				subprocess.config.ExecutablePath,
				subprocess.config.Args,
				subprocess.envVars,
			)

			go collectStdout(bufio.NewScanner(stdout), subprocess.Stdout, subprocess.logger)

			subprocess.logger.Debug("starting subprocess", zap.String("command", cmd.String()))
			err = cmd.Start()
			if err != nil {
				state = errored
				continue
			}
			subprocess.pid.setPid(cmd.Process.Pid)

			go signalWhenProcessReturned(cmd, processReturned)

			state = running
		case running:
			err = subprocess.sendToStdIn(subprocess.config.StdInContents, stdin)
			stdin.Close()
			if err != nil {
				state = errored
				continue
			}

			select {
			case err = <-processReturned.ReturnedChan:
				if err != nil && ctx.Err() == nil {
					err = fmt.Errorf("unexpected shutdown: %w", err)
					// We aren't supposed to shutdown yet so this is an error state.
					state = errored
					continue
				}
				// We must close this channel or can wait indefinitely at shuttingDown
				processReturned.close()
				state = shuttingDown
			case <-ctx.Done(): // context-based cancel.
				state = shuttingDown
			}
		case errored:
			subprocess.logger.Error("subprocess died", zap.Error(err))
			if subprocess.config.RestartOnError {
				subprocess.pid.setPid(-1)
				state = restarting
			} else {
				// We must close this channel or can wait indefinitely at shuttingDown
				processReturned.close()
				state = shuttingDown
			}
		case shuttingDown:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}
			<-processReturned.ReturnedChan
			stdout.Close()
			subprocess.pid.setPid(-1)
			state = stopped
		case restarting:
			stdout.Close()
			stdin.Close()
			time.Sleep(*subprocess.config.RestartDelay)
			state = starting
		case stopped:
			return
		}
	}
}

func signalWhenProcessReturned(cmd *exec.Cmd, pr *processReturned) {
	err := cmd.Wait()
	pr.signal(err)
}

func collectStdout(stdoutScanner *bufio.Scanner, stdoutChan chan<- string, logger *zap.Logger) {
	for stdoutScanner.Scan() {
		text := stdoutScanner.Text()
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
	env = append(env, envVars...)
	cmd.Env = env

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
