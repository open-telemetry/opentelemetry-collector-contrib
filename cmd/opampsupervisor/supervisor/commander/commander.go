// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package commander

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// Commander can start/stop/restart the Agent executable and also watch for a signal
// for the Agent process to finish.
type Commander struct {
	logger  *zap.Logger
	cfg     config.Agent
	logsDir string
	args    []string
	cmd     *exec.Cmd
	doneCh  chan struct{}
	exitCh  chan struct{}
	running *atomic.Int64
}

func NewCommander(logger *zap.Logger, logsDir string, cfg config.Agent, args ...string) (*Commander, error) {
	return &Commander{
		logger:  logger,
		logsDir: logsDir,
		cfg:     cfg,
		args:    args,
		running: &atomic.Int64{},
		// Buffer channels so we can send messages without blocking on listeners.
		doneCh: make(chan struct{}, 1),
		exitCh: make(chan struct{}, 1),
	}, nil
}

// Start the Agent and begin watching the process.
// Agent's stdout and stderr are written to a file.
// Calling this method when a command is already running
// is a no-op.
func (c *Commander) Start(ctx context.Context) error {
	if c.running.Load() == 1 {
		// Already started, nothing to do
		return nil
	}

	// Drain channels in case there are no listeners that
	// drained messages from previous runs.
	if len(c.doneCh) > 0 {
		select {
		case <-c.doneCh:
		default:
		}
	}
	if len(c.exitCh) > 0 {
		select {
		case <-c.exitCh:
		default:
		}
	}
	c.logger.Debug("Starting agent", zap.String("agent", c.cfg.Executable))

	args := slices.Concat(c.args, c.cfg.Arguments)

	c.cmd = exec.CommandContext(ctx, c.cfg.Executable, args...) // #nosec G204
	c.cmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	c.cmd.SysProcAttr = sysProcAttrs()

	// PassthroughLogging changes how collector start up happens
	if c.cfg.PassthroughLogs {
		return c.startWithPassthroughLogging()
	}
	return c.startNormal()
}

func (c *Commander) Restart(ctx context.Context) error {
	c.logger.Debug("Restarting agent", zap.String("agent", c.cfg.Executable))
	if err := c.Stop(ctx); err != nil {
		return err
	}

	return c.Start(ctx)
}

func (c *Commander) startNormal() error {
	logFilePath := filepath.Join(c.logsDir, "agent.log")
	stdoutFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("cannot create %s: %w", logFilePath, err)
	}

	// Capture standard output and standard error.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21072
	c.cmd.Stdout = stdoutFile
	c.cmd.Stderr = stdoutFile

	if err := c.cmd.Start(); err != nil {
		stdoutFile.Close()
		return fmt.Errorf("startNormal: %w", err)
	}

	c.logger.Debug("Agent process started", zap.Int("pid", c.cmd.Process.Pid))
	c.running.Store(1)

	go func() {
		defer stdoutFile.Close()
		c.watch()
	}()

	return nil
}

func (c *Commander) startWithPassthroughLogging() error {
	// grab cmd pipes
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdoutPipe: %w", err)
	}
	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderrPipe: %w", err)
	}

	// start agent
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	c.running.Store(1)

	colLogger := c.logger.Named("collector")

	// capture agent output
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			colLogger.Info(line)
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stdout: %w", zap.Error(err))
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			colLogger.Info(line)
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stderr: %w", zap.Error(err))
		}
	}()

	c.logger.Debug("Agent process started", zap.Int("pid", c.cmd.Process.Pid))

	go c.watch()
	return nil
}

func (c *Commander) watch() {
	err := c.cmd.Wait()

	// cmd.Wait returns an exec.ExitError when the Collector exits unsuccessfully or stops
	// after receiving a signal. The Commander caller will handle these cases, so we filter
	// them out here.
	var exitError *exec.ExitError
	if ok := errors.As(err, &exitError); err != nil && !ok {
		c.logger.Error("An error occurred while watching the agent process", zap.Error(err))
	}

	c.running.Store(0)
	c.doneCh <- struct{}{}
	c.exitCh <- struct{}{}
}

// StartOneShot starts the Collector with the expectation that it will immediately
// exit after it finishes a quick operation. This is useful for situations like reading stdout/sterr
// to e.g. check the feature gate the Collector supports.
func (c *Commander) StartOneShot() ([]byte, []byte, error) {
	stdout := []byte{}
	stderr := []byte{}
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, c.cfg.Executable, c.args...) // #nosec G204
	cmd.Env = common.EnvVarMapToEnvMapSlice(c.cfg.Env)
	cmd.SysProcAttr = sysProcAttrs()
	// grab cmd pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdoutPipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderrPipe: %w", err)
	}

	// start agent
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start: %w", err)
	}
	// capture agent output
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			stdout = append(stdout, scanner.Bytes()...)
			stdout = append(stdout, byte('\n'))
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stdout: %w", zap.Error(err))
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			stderr = append(stderr, scanner.Bytes()...)
			stderr = append(stderr, byte('\n'))
		}
		if err := scanner.Err(); err != nil {
			c.logger.Error("Error reading agent stderr: %w", zap.Error(err))
		}
	}()

	c.logger.Debug("Agent process started", zap.Int("pid", cmd.Process.Pid))

	doneCh := make(chan struct{}, 1)

	go func() {
		err := cmd.Wait()
		if err != nil {
			c.logger.Error("One-shot Collector encountered an error during execution", zap.Error(err))
		}
		doneCh <- struct{}{}
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)

	defer cancel()

	select {
	case <-doneCh:
	case <-waitCtx.Done():
		pid := cmd.Process.Pid
		c.logger.Debug("Stopping agent process", zap.Int("pid", pid))

		// Gracefully signal process to stop.
		if err := sendShutdownSignal(cmd.Process); err != nil {
			return nil, nil, err
		}

		innerWaitCtx, innerCancel := context.WithTimeout(ctx, 10*time.Second)

		// Setup a goroutine to wait a while for process to finish and send kill signal
		// to the process if it doesn't finish.
		var innerErr error
		go func() {
			<-innerWaitCtx.Done()

			if !errors.Is(innerWaitCtx.Err(), context.DeadlineExceeded) {
				c.logger.Debug("Agent process successfully stopped.", zap.Int("pid", pid))
				return
			}

			// Time is out. Kill the process.
			c.logger.Debug(
				"Agent process is not responding to SIGTERM. Sending SIGKILL to kill forcibly.",
				zap.Int("pid", pid))
			if innerErr = cmd.Process.Signal(os.Kill); innerErr != nil {
				return
			}
		}()

		innerCancel()
	}

	return stdout, stderr, nil
}

// Exited returns a channel that will send a signal when the Agent process exits.
func (c *Commander) Exited() <-chan struct{} {
	return c.exitCh
}

// Pid returns Agent process PID if it is started or 0 if it is not.
func (c *Commander) Pid() int {
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

// ExitCode returns Agent process exit code if it exited or 0 if it is not.
func (c *Commander) ExitCode() int {
	if c.cmd == nil || c.cmd.ProcessState == nil {
		return 0
	}
	return c.cmd.ProcessState.ExitCode()
}

func (c *Commander) IsRunning() bool {
	return c.running.Load() != 0
}

// Stop the Agent process. Sends SIGTERM to the process and wait for up 10 seconds
// and if the process does not finish kills it forcedly by sending SIGKILL.
// Returns after the process is terminated.
func (c *Commander) Stop(ctx context.Context) error {
	if c.running.Load() == 0 {
		// Not started, nothing to do.
		return nil
	}

	pid := c.cmd.Process.Pid
	c.logger.Debug("Stopping agent process", zap.Int("pid", pid))

	// Gracefully signal process to stop.
	if err := sendShutdownSignal(c.cmd.Process); err != nil {
		return err
	}

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

	// Setup a goroutine to wait a while for process to finish and send kill signal
	// to the process if it doesn't finish.
	var innerErr error
	go func() {
		<-waitCtx.Done()

		if !errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
			c.logger.Debug("Agent process successfully stopped.", zap.Int("pid", pid))
			return
		}

		// Time is out. Kill the process.
		c.logger.Debug(
			"Agent process is not responding to SIGTERM. Sending SIGKILL to kill forcibly.",
			zap.Int("pid", pid))
		if innerErr = c.cmd.Process.Signal(os.Kill); innerErr != nil {
			return
		}
	}()

	// Wait for process to terminate
	<-c.doneCh

	c.running.Store(0)

	// Let goroutine know process is finished.
	cancel()

	return innerErr
}
