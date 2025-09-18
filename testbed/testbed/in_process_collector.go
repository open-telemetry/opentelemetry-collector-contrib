// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
)

// inProcessCollector implements the OtelcolRunner interfaces running a single otelcol as a go routine within the
// same process as the test executor.
type inProcessCollector struct {
	factories  otelcol.Factories
	configStr  string
	svc        *otelcol.Collector
	stopped    bool
	configFile string
	wg         sync.WaitGroup
	t          *testing.T
}

// NewInProcessCollector creates a new inProcessCollector using the supplied component factories.
func NewInProcessCollector(factories otelcol.Factories) OtelcolRunner {
	return &inProcessCollector{
		factories: factories,
	}
}

func (ipp *inProcessCollector) PrepareConfig(t *testing.T, configStr string) (configCleanup func(), err error) {
	configCleanup = func() {
		// NoOp
	}
	ipp.configStr = configStr
	ipp.t = t
	return configCleanup, err
}

func (ipp *inProcessCollector) Start(StartParams) error {
	var err error

	var dir string
	if runtime.GOOS == "windows" {
		// On Windows, use os.MkdirTemp to create directory since t.TempDir results in an error during cleanup in scoped-tests.
		// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42639
		var err error
		dir, err = os.MkdirTemp("", ipp.t.Name()) //nolint:usetesting
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
	} else {
		dir = ipp.t.TempDir()
	}

	confFile, err := os.CreateTemp(dir, "conf-")
	if err != nil {
		return err
	}

	if _, err = confFile.WriteString(ipp.configStr); err != nil {
		os.Remove(confFile.Name())
		return err
	}
	ipp.configFile = confFile.Name()

	settings := otelcol.CollectorSettings{
		BuildInfo: component.NewDefaultBuildInfo(),
		Factories: func() (otelcol.Factories, error) { return ipp.factories, nil },
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:              []string{ipp.configFile},
				ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory()},
			},
		},
		SkipSettingGRPCLogger: true,
	}

	ipp.svc, err = otelcol.NewCollector(settings)
	if err != nil {
		return err
	}

	ipp.wg.Add(1)
	go func() {
		defer ipp.wg.Done()
		if appErr := ipp.svc.Run(context.Background()); appErr != nil {
			// TODO: Pass this to the error handler.
			panic(appErr)
		}
	}()

	for {
		switch state := ipp.svc.GetState(); state {
		case otelcol.StateStarting:
			time.Sleep(time.Second)
		case otelcol.StateRunning:
			return nil
		default:
			return fmt.Errorf("unable to start, otelcol state is %d", state)
		}
	}
}

func (ipp *inProcessCollector) Stop() (stopped bool, err error) {
	if !ipp.stopped {
		ipp.stopped = true
		ipp.svc.Shutdown()
		// Retry deleting files on windows because in windows several processes read file handles.
		if runtime.GOOS == "windows" {
			require.Eventually(ipp.t, func() bool {
				err = os.Remove(ipp.configFile)
				if errors.Is(err, os.ErrNotExist) {
					err = nil
					return true
				}
				return false
			}, 5*time.Second, 100*time.Millisecond)
		}
	}
	ipp.wg.Wait()
	stopped = ipp.stopped
	return stopped, err
}

func (*inProcessCollector) WatchResourceConsumption() error {
	return nil
}

func (*inProcessCollector) GetProcessMon() *process.Process {
	return nil
}

func (*inProcessCollector) GetTotalConsumption() *ResourceConsumption {
	return &ResourceConsumption{
		CPUPercentAvg:   0,
		CPUPercentMax:   0,
		CPUPercentLimit: 0,
		RAMMiBAvg:       0,
		RAMMiBMax:       0,
		RAMMiBLimit:     0,
	}
}

func (*inProcessCollector) GetResourceConsumption() string {
	return ""
}
