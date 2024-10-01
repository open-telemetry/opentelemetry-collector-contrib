// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"os/signal"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func main() {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	logger, _ := zap.NewDevelopment()

	cfg, err := config.Load(*configFlag)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	supervisor, err := supervisor.NewSupervisor(logger, cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	err = supervisor.Start()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(-1)
		return
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	supervisor.Shutdown()
}
