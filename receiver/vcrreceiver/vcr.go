// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcrreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcrreceiver"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
)

type rawFile struct {
	path       string
	signalType string
	sequence   int
}

type tapeFile struct {
	path    string
	startNs int64
	endNs   int64
}

type tapeLoader struct {
	dir         []string
	loadedFiles map[string]tapeFile
	mu          sync.Mutex
}

type Config struct {
	IncludeRaw  []string `mapstructure:"include_raw"`
	ExcludeRaw  []string `mapstructure:"exclude_raw"`
	IncludeTape []string `mapstructure:"include_tape"`
}

func createDefaultConfig() component.Config {
	return &Config{
		IncludeRaw:  []string{},
		ExcludeRaw:  []string{},
		IncludeTape: []string{"."}, // default to current directory
	}
}

type vcrReceiver struct {
	loader *tapeLoader
	cancel context.CancelFunc
}

func NewFactory() receiver.Factory {
	typ, err := component.NewType("vcrreceiver")
	if err != nil {
		panic(err)
	}

	return xreceiver.NewFactory(
		typ,
		createDefaultConfig,
		xreceiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
		xreceiver.WithLogs(createLogsReceiver, component.StabilityLevelDevelopment),
		xreceiver.WithTraces(createTracesReceiver, component.StabilityLevelDevelopment),
		xreceiver.WithProfiles(createProfilesReceiver, component.StabilityLevelDevelopment))
}

func (v *vcrReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (v *vcrReceiver) Shutdown(ctx context.Context) error {
	v.cancel()
	return nil
}

func newTapeLoader(dir []string) *tapeLoader {
	return &tapeLoader{
		dir:         dir,
		loadedFiles: make(map[string]tapeFile),
	}
}

func createMetricsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	metrics consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	loader := newTapeLoader(c.IncludeTape)

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr metrics playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createTracesReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	traces consumer.Traces,
) (receiver.Traces, error) {
	c := cfg.(*Config)
	loader := newTapeLoader(c.IncludeTape)

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr traces playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createLogsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	logs consumer.Logs,
) (receiver.Logs, error) {
	c := cfg.(*Config)
	loader := newTapeLoader(c.IncludeTape)

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr logs playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createProfilesReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	profiles xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	c := cfg.(*Config)
	loader := newTapeLoader(c.IncludeTape)

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr profiles playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}
