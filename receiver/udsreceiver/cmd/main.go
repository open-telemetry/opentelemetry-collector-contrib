// Copyright 2019, OpenTelemetry Authors
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

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"contrib.go.opencensus.io/exporter/ocagent"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.opencensus.io/zpages"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver/processor/local"
)

const (
	serviceName = "OCDaemon-PHP"

	defaultOCAgentAddr      = "localhost:55678"
	defaultLogLevel         = logError
	defaultMsgBufSize       = 10000
	defaultZPagesAddr       = ""
	defaultZPagesPathPrefix = "/debug"
)

const (
	logNone = iota
	logError
	logWarn
	logInfo
	logDebug
)

// build values injected at compile time (see Makefile)
var (
	buildVersion string
	buildDate    string
)

func main() {
	var (
		fs                   = flag.NewFlagSet("", flag.ExitOnError)
		flagOCAgentAddr      = fs.String("ocagent.addr", defaultOCAgentAddr, "Address of the OpenCensus Agent")
		flagServiceName      = fs.String("php.servicename", os.Getenv("HOSTNAME"), "Name of our PHP service")
		flagLogLevel         = fs.Int("log.level", defaultLogLevel, "Logging level to use")
		flagMsgBufSize       = fs.Int("msg.bufsize", defaultMsgBufSize, "Size of buffered message channel")
		flagMsgProcCount     = fs.Int("msg.processors", runtime.NumCPU(), "Amount of message processing routines to use")
		flagZPagesAddr       = fs.String("zpages.addr", defaultZPagesAddr, "zPages bind address")
		flagZPagesPathPrefix = fs.String("zpages.path", defaultZPagesPathPrefix, "zPages path prefix")
		flagVersion          = fs.Bool("version", false, "Show version information")
		flagTransportPath    = addTransportPath(fs) // transport path flag conditional on target platform
	)

	// parse our command line override flags
	if err := fs.Parse(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}

	if *flagVersion {
		fmt.Printf(
			"Version information for: %s\nBuild     : %s\nBuild date: %s\n\n",
			serviceName,
			buildVersion,
			buildDate,
		)
		os.Exit(0)
	}

	// set-up our structured logger
	logger := initLogger(*flagLogLevel)
	_ = level.Info(logger).Log("msg", "service started", "agent", *flagOCAgentAddr)
	defer func() {
		_ = level.Info(logger).Log("msg", "service ended")
	}()

	// Use a goroutine lifecycle manager for our services.
	// See: https://github.com/oklog/run
	g := &run.Group{}

	// initialize and register our zPages service
	if err := svcZPages(g, *flagZPagesAddr, *flagZPagesPathPrefix); err != nil {
		_ = level.Error(logger).Log("msg", "unable to start zPages", "err", err)
	}

	// initialize and add our daemon message processing service
	hnd, err := svcProcessor(g,
		*flagMsgBufSize, *flagMsgProcCount, *flagServiceName, *flagOCAgentAddr, logger,
	)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to create OCAgent exporter", "err", err)
		os.Exit(1)
	}

	// initialize and add our transport service
	// linux/darwin: unix_socket & windows: named pipes
	svcTransport(g, hnd, *flagTransportPath)

	// initialize our signal handler for enabling graceful shutdown
	signalHandler(g, logger)

	// spawn our service goroutines and wait for shutdown
	if err := g.Run(); err != nil {
		_ = level.Error(logger).Log("msg", "unexpected shutdown of service", "err", err)
	}
}

func initLogger(lvl int) (logger log.Logger) {
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	logger = log.With(logger, "svc", serviceName, "ts", log.DefaultTimestampUTC, "clr", log.DefaultCaller)

	switch lvl {
	case logNone:
		logger = log.NewNopLogger()
	case logError:
		logger = level.NewFilter(logger, level.AllowError())
	case logWarn:
		logger = level.NewFilter(logger, level.AllowWarn())
	case logInfo:
		logger = level.NewFilter(logger, level.AllowInfo())
	case logDebug:
		logger = level.NewFilter(logger, level.AllowDebug())
	default:
		logger = level.NewFilter(logger, level.AllowError())
	}
	return
}

func svcProcessor(g *run.Group, msgBufSize, msgProcCount int, svcName, ocAgentAddr string, logger log.Logger) (udsreceiver.Handler, error) {
	if msgBufSize < 100 {
		msgBufSize = defaultMsgBufSize
	}

	if msgProcCount < 1 {
		msgProcCount = runtime.NumCPU()
	}

	if svcName == "" {
		svcName = serviceName
	}

	var (
		err      error
		exporter interface {
			view.Exporter
			trace.Exporter
		}
	)

	exporter, err = ocagent.NewExporter(
		ocagent.WithInsecure(),
		ocagent.WithAddress(ocAgentAddr),
		ocagent.WithServiceName(svcName),
	)

	if err != nil {
		return nil, err
	}

	// enables Daemon internal traces
	trace.RegisterExporter(exporter)

	// enables Daemon and PHP Process Views
	view.RegisterExporter(exporter)

	// provide agent as exporter for proxied spans
	processor := local.New(msgBufSize, msgProcCount, []trace.Exporter{exporter}, logger)

	g.Add(func() error {
		return processor.Run()
	}, func(error) {
		_ = processor.Close()
		if agentExporter, ok := exporter.(*ocagent.Exporter); ok {
			agentExporter.Flush()
		}
	})

	return &udsreceiver.ConnectionHandler{Logger: logger, Processor: processor}, nil
}

func svcZPages(g *run.Group, bindAddr string, pathPrefix string) error {
	if bindAddr == "" {
		// without a bind address we disable zPages
		return nil
	}

	mux := http.NewServeMux()
	zpages.Handle(mux, pathPrefix)

	l, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return err
	}

	g.Add(func() error {
		return http.Serve(l, mux)
	}, func(error) {
		_ = l.Close()
	})

	return nil
}

func signalHandler(g *run.Group, logger log.Logger) {
	var (
		cInt = make(chan struct{})
		cSig = make(chan os.Signal, 2)
	)

	g.Add(func() error {
		signal.Notify(cSig, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-cSig:
			_ = level.Info(logger).Log("msg", "shutdown requested", "signal", sig)
			return nil
		case <-cInt:
			return nil
		}
	}, func(error) {
		close(cInt)
		close(cSig)
	})
}
