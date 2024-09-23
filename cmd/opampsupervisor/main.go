// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func runInteractive() error {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Fatal("failed to load config: %w", err)
	}

	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	if err != nil {
		log.Fatal("failed to create logger: %w", err)
	}

	supervisor, err := supervisor.NewSupervisor(logger, cfg)
	if err != nil {
		return err
	}

	err = supervisor.Start()
	if err != nil {
		return err
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	supervisor.Shutdown()

	return nil
}
