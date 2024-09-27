// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
)

func main() {
	configFlag := flag.String("config", "", "Path to a supervisor configuration file")
	flag.Parse()

	// load & validate config
	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Fatal("failed to load config: %w", err)
	}

	// create logger
	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	if err != nil {
		log.Fatal("failed to create logger: %w", err)
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

func loadConfig(configFile string) (config.Supervisor, error) {
	if configFile == "" {
		return config.Supervisor{}, errors.New("path to config file cannot be empty")
	}

	k := koanf.New("::")
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return config.Supervisor{}, err
	}

	decodeConf := koanf.UnmarshalConf{
		Tag: "mapstructure",
	}

	cfg := config.DefaultSupervisor()
	if err := k.UnmarshalWithConf("", &cfg, decodeConf); err != nil {
		return config.Supervisor{}, fmt.Errorf("cannot parse %v: %w", configFile, err)
	}

	return cfg, nil
}
