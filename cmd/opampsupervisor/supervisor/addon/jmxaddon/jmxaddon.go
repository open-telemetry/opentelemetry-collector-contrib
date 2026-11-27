// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package jmxaddon implements the "jmx" addon type: the OpenTelemetry JMX
// Metric Scraper (io.opentelemetry.contrib:opentelemetry-jmx-scraper), run as a
// Supervisor-managed child process that exports OTLP to the Collector.
//
// The package content is the scraper jar exactly as published, so a server can
// offer the upstream release asset and its SHA-256 without repackaging it. The
// type, not the package, decides how the jar is run: the server cannot make the
// Supervisor execute anything other than "java -jar <scraper> -config <file>".
package jmxaddon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon/process"
)

// Type is the addon type implemented by this package.
const Type addon.Type = "jmx"

const (
	jarFile    = "opentelemetry-jmx-scraper.jar"
	configFile = "scraper.properties"
	logFile    = "addon.log"

	stopGracePeriod = 10 * time.Second
)

// zipMagic starts every jar, which is a zip archive.
var zipMagic = []byte("PK\x03\x04")

// NewFactory returns the factory for jmx addons.
func NewFactory() addon.Factory {
	return factory{}
}

type factory struct{}

func (factory) Type() addon.Type { return Type }

// Install stores the scraper jar after checking that the host can run it. The
// JVM check happens here rather than at start so that a host without Java
// reports INSTALL_FAILED to the server, instead of reporting a successful
// install of an addon that can never run.
func (factory) Install(_ context.Context, set addon.InstallSettings) error {
	if _, err := findJava(); err != nil {
		return err
	}
	if !bytes.HasPrefix(set.Content, zipMagic) {
		return errors.New("jmx addon package must be the opentelemetry-jmx-scraper jar")
	}
	if err := os.WriteFile(filepath.Join(set.Dir, jarFile), set.Content, 0o600); err != nil {
		return fmt.Errorf("write scraper jar: %w", err)
	}
	return nil
}

func (factory) Create(_ context.Context, set addon.Settings) (addon.Addon, error) {
	return &jmxAddon{
		id:     set.ID,
		dir:    set.Dir,
		logger: set.Logger,
	}, nil
}

type jmxAddon struct {
	id     addon.ID
	dir    string
	logger *zap.Logger

	mu   sync.Mutex
	proc *process.Process
}

// Configure writes the scraper's properties file. The scraper reads it only at
// startup, so a running scraper is restarted. Without configuration the
// scraper cannot run, since it needs at least a JMX service URL, so removing
// the configuration stops it.
func (a *jmxAddon) Configure(_ context.Context, body []byte) (addon.Effect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := filepath.Join(a.dir, configFile)

	if body == nil {
		if err := a.stopLocked(); err != nil {
			return addon.Effect{}, err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return addon.Effect{}, fmt.Errorf("remove scraper config: %w", err)
		}
		return addon.Effect{}, nil
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return addon.Effect{}, fmt.Errorf("write scraper config: %w", err)
	}

	if a.proc != nil && a.proc.IsRunning() {
		a.logger.Info("Restarting JMX scraper to apply new configuration")
		if err := a.stopLocked(); err != nil {
			return addon.Effect{}, err
		}
		if err := a.startLocked(); err != nil {
			return addon.Effect{}, err
		}
	}

	return addon.Effect{}, nil
}

// Start runs the scraper once it has configuration. An unconfigured scraper is
// installed but idle, which is not an error: its configuration may simply not
// have arrived yet.
func (a *jmxAddon) Start(context.Context) (addon.Effect, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.proc != nil && a.proc.IsRunning() {
		return addon.Effect{}, nil
	}
	if _, err := os.Stat(filepath.Join(a.dir, configFile)); errors.Is(err, os.ErrNotExist) {
		a.logger.Info("JMX scraper is installed but has no configuration; not starting it")
		return addon.Effect{}, nil
	}

	return addon.Effect{}, a.startLocked()
}

func (a *jmxAddon) Shutdown(context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopLocked()
}

func (a *jmxAddon) startLocked() error {
	java, err := findJava()
	if err != nil {
		return err
	}

	a.proc = process.New(a.logger, process.Settings{
		Executable:  java,
		Args:        []string{"-jar", filepath.Join(a.dir, jarFile), "-config", filepath.Join(a.dir, configFile)},
		Dir:         a.dir,
		LogFilePath: filepath.Join(a.dir, logFile),
		GracePeriod: stopGracePeriod,
	})
	return a.proc.Start()
}

func (a *jmxAddon) stopLocked() error {
	if a.proc == nil {
		return nil
	}
	return a.proc.Stop()
}

// IsRunning reports whether the scraper process is running. It and Pid are
// not part of addon.Addon; callers that need them type-assert.
func (a *jmxAddon) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.proc != nil && a.proc.IsRunning()
}

// Pid returns the scraper's process id, or 0 when it is not running.
func (a *jmxAddon) Pid() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return 0
	}
	return a.proc.Pid()
}

// findJava locates the JVM the same way the java launcher conventions do:
// JAVA_HOME first, then PATH.
func findJava() (string, error) {
	name := "java"
	if runtime.GOOS == "windows" {
		name = "java.exe"
	}

	if home := os.Getenv("JAVA_HOME"); home != "" {
		candidate := filepath.Join(home, "bin", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: the jmx addon needs a Java runtime; install one on the host and put it on PATH or set JAVA_HOME", addon.ErrRuntimeNotFound)
	}
	return path, nil
}
