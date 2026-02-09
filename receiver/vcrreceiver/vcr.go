// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcrreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcrreceiver"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"
)

/*type rawFile struct {
	path       string
	signalType string
	sequence   int
}*/

type tapeFile struct {
	path    string
	startNs int64
	endNs   int64
}

type tapeLoader struct {
	dir         []string
	signalType  string
	loadedFiles map[string]tapeFile
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
}

func NewFactory() receiver.Factory {
	typ, err := component.NewType("vcr")
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

func (*vcrReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*vcrReceiver) Shutdown(_ context.Context) error {
	return nil
}

func newTapeLoader(dir []string, signalType string) (*tapeLoader, error) {
	loader := &tapeLoader{
		dir:         dir,
		signalType:  signalType,
		loadedFiles: make(map[string]tapeFile),
	}

	if err := loader.loadTapeFiles(); err != nil {
		return nil, err
	}
	return loader, nil
}

func (t *tapeLoader) loadTapeFiles() error {
	pattern := regexp.MustCompile(fmt.Sprintf(`^tape_%s_(\d{18,20})-(\d{18,20})\.json$`, t.signalType))
	for _, dir := range t.dir {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := pattern.FindStringSubmatch(entry.Name())
			if matches == nil {
				// Skip files that don't match the pattern
				continue
			}
			// matches[0] = full string
			// matches[1] = signal type (metrics|traces|logs|profiles)
			// matches[2] = start timestamp
			// matches[3] = end timestamp
			startNs, err := strconv.ParseInt(matches[2], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid start timestamp in file %s: %w", entry.Name(), err)
			}
			endNs, err := strconv.ParseInt(matches[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid end timestamp in file %s: %w", entry.Name(), err)
			}
			if endNs < startNs {
				return fmt.Errorf("end timestamp must be >= start timestamp in file %s", entry.Name())
			}
			fullPath := filepath.Join(dir, entry.Name())
			t.loadedFiles[fullPath] = tapeFile{
				path:    fullPath,
				startNs: startNs,
				endNs:   endNs,
			}
		}
	}
	return nil
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	_ consumer.Metrics,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	loader, err := newTapeLoader(c.IncludeTape, "metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to create tape loader: %w", err)
	}

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr metrics playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createTracesReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	_ consumer.Traces,
) (receiver.Traces, error) {
	c := cfg.(*Config)
	loader, err := newTapeLoader(c.IncludeTape, "traces")
	if err != nil {
		return nil, fmt.Errorf("failed to create tape loader: %w", err)
	}

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr traces playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	_ consumer.Logs,
) (receiver.Logs, error) {
	c := cfg.(*Config)
	loader, err := newTapeLoader(c.IncludeTape, "logs")
	if err != nil {
		return nil, fmt.Errorf("failed to create tape loader: %w", err)
	}

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr logs playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}

func createProfilesReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	_ xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	c := cfg.(*Config)
	loader, err := newTapeLoader(c.IncludeTape, "profiles")
	if err != nil {
		return nil, fmt.Errorf("failed to create tape loader: %w", err)
	}

	go func() {
		// start playback tape loop here
		settings.Logger.Info("vcr profiles playback loop in development.")
	}()

	return &vcrReceiver{
		loader: loader,
	}, nil
}
