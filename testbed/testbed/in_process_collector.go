// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
	confFile, err := os.CreateTemp(testutil.TempDir(ipp.t), "conf-")
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

	ipp.wg.Go(func() {
		if appErr := ipp.svc.Run(context.Background()); appErr != nil {
			// TODO: Pass this to the error handler.
			panic(appErr)
		}
	})

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
		// Do not delete temporary files on Windows because it fails too much on scoped tests.
		// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42639
		if runtime.GOOS != "windows" {
			require.NoError(ipp.t, os.Remove(ipp.configFile))
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
