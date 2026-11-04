// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package process runs a long-lived child process for an addon type that needs
// one. It is exported so that addon types implemented outside this module can
// reuse it.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Settings describes the process to run.
type Settings struct {
	// Executable is the absolute path of the program to run.
	Executable string
	Args       []string
	// Env is the complete environment of the process. A nil Env inherits the
	// Supervisor's environment.
	Env []string
	// Dir is the working directory of the process.
	Dir string
	// LogFilePath receives the process's stdout and stderr.
	LogFilePath string
	// GracePeriod is how long the process has to exit after being asked to
	// before it is killed.
	GracePeriod time.Duration
}

// Process is a managed child process.
//
// The Supervisor's Commander already does this for the Collector, but its
// semantics are specific to it: it signals with SIGINT, which a Go binary
// handles, and it does not put the child in its own process group because the
// Collector has no children of its own. Neither holds for addons, which are
// commonly a runtime or a wrapper script around one. A child that ignores SIGINT
// is only killed after the whole grace period, so every restart costs that
// long. Process runs the child in its own process group and signals the group
// with SIGTERM.
type Process struct {
	logger   *zap.Logger
	settings Settings

	// mu serializes Start and Stop.
	mu      sync.Mutex
	cmd     *exec.Cmd
	running atomic.Bool
	// exited is closed once the child has been reaped.
	exited chan struct{}
}

// New returns a Process that is not yet running.
func New(logger *zap.Logger, settings Settings) *Process {
	return &Process{logger: logger, settings: settings}
}

// Start launches the process. Starting a running process is a no-op.
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running.Load() {
		return nil
	}

	logFile, err := os.OpenFile(p.settings.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open process log file: %w", err)
	}

	// Not CommandContext: the process outlives the message that started it and
	// is stopped explicitly.
	cmd := exec.Command(p.settings.Executable, p.settings.Args...) // #nosec G204 -- the executable is resolved by the addon type, not taken from the server
	cmd.Env = p.settings.Env
	cmd.Dir = p.settings.Dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = sysProcAttr()

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	p.cmd = cmd
	p.running.Store(true)
	p.exited = make(chan struct{})

	exited := p.exited
	go func() {
		defer logFile.Close()
		if err := cmd.Wait(); err != nil {
			if _, ok := errors.AsType[*exec.ExitError](err); !ok {
				p.logger.Error("Error waiting for addon process", zap.Error(err))
			}
		}
		p.running.Store(false)
		close(exited)
	}()

	p.logger.Info("Addon process started", zap.Int("pid", cmd.Process.Pid), zap.String("executable", p.settings.Executable))
	return nil
}

// Stop signals the process group and waits for it to exit, killing it if it
// has not exited within the grace period. Stopping a process that is not
// running is a no-op.
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || !p.running.Load() {
		return nil
	}

	exited := p.exited
	pid := p.cmd.Process.Pid

	if err := terminate(p.cmd.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
		p.logger.Debug("Could not signal addon process", zap.Int("pid", pid), zap.Error(err))
	}

	timer := time.NewTimer(p.settings.GracePeriod)
	defer timer.Stop()

	select {
	case <-exited:
		p.logger.Info("Addon process stopped", zap.Int("pid", pid))
		return nil
	case <-timer.C:
	}

	p.logger.Warn("Addon process did not exit within the grace period, killing it",
		zap.Int("pid", pid), zap.Duration("grace_period", p.settings.GracePeriod))

	if err := kill(p.cmd.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill addon process: %w", err)
	}

	select {
	case <-exited:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("addon process %d did not exit after being killed", pid)
	}
}

// IsRunning reports whether the child is running.
func (p *Process) IsRunning() bool {
	return p.running.Load()
}

// Pid returns the child's process id, or 0 when it is not running.
func (p *Process) Pid() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil || !p.running.Load() {
		return 0
	}
	return p.cmd.Process.Pid
}
