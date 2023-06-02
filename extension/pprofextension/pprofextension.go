// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"

import (
	"context"
	"errors"
	"net"
	"net/http"
	_ "net/http/pprof" // #nosec Needed to enable the performance profiler
	"os"
	"runtime"
	"runtime/pprof"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var running = &atomic.Bool{}

type pprofExtension struct {
	config Config
	logger *zap.Logger
	file   *os.File
	server http.Server
	stopCh chan struct{}
}

func (p *pprofExtension) Start(_ context.Context, host component.Host) error {
	// The runtime settings are global to the application, so while in principle it
	// is possible to have more than one instance, running multiple will mean that
	// the settings of the last started instance will prevail. In order to avoid
	// this issue we will allow the start of a single instance once per process
	// Summary: only a single instance can be running in the same process.
	if !running.CompareAndSwap(false, true) {
		return errors.New("only a single pprof extension instance can be running per process")
	}

	// Take care that if any error happen when starting the active instance is cleaned.
	var startErr error
	defer func() {
		if startErr != nil {
			running.Store(false)
		}
	}()

	// Start the listener here so we can have earlier failure if port is
	// already in use.
	var ln net.Listener
	ln, startErr = p.config.TCPAddr.Listen()
	if startErr != nil {
		return startErr
	}

	runtime.SetBlockProfileRate(p.config.BlockProfileFraction)
	runtime.SetMutexProfileFraction(p.config.MutexProfileFraction)

	p.logger.Info("Starting net/http/pprof server", zap.Any("config", p.config))
	p.stopCh = make(chan struct{})
	go func() {
		defer func() {
			running.Store(false)
			close(p.stopCh)
		}()

		// The listener ownership goes to the server.
		if errHTTP := p.server.Serve(ln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()

	if p.config.SaveToFile != "" {
		var f *os.File
		f, startErr = os.Create(p.config.SaveToFile)
		if startErr != nil {
			return startErr
		}
		p.file = f
		startErr = pprof.StartCPUProfile(f)
	}

	return startErr
}

func (p *pprofExtension) Shutdown(context.Context) error {
	defer running.Store(false)
	if p.file != nil {
		pprof.StopCPUProfile()
		_ = p.file.Close() // ignore the error
	}
	err := p.server.Close()
	if p.stopCh != nil {
		<-p.stopCh
	}
	return err
}

func newServer(config Config, logger *zap.Logger) *pprofExtension {
	return &pprofExtension{
		config: config,
		logger: logger,
	}
}
