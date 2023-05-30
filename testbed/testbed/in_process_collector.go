// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
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
}

// NewInProcessCollector creates a new inProcessCollector using the supplied component factories.
func NewInProcessCollector(factories otelcol.Factories) OtelcolRunner {
	return &inProcessCollector{
		factories: factories,
	}
}

func (ipp *inProcessCollector) PrepareConfig(configStr string) (configCleanup func(), err error) {
	configCleanup = func() {
		// NoOp
	}
	ipp.configStr = configStr
	return configCleanup, err
}

func (ipp *inProcessCollector) Start(_ StartParams) error {
	var err error

	confFile, err := os.CreateTemp(os.TempDir(), "conf-")
	if err != nil {
		return err
	}

	if _, err = confFile.Write([]byte(ipp.configStr)); err != nil {
		os.Remove(confFile.Name())
		return err
	}
	ipp.configFile = confFile.Name()

	fmp := fileprovider.New()
	configProvider, err := otelcol.NewConfigProvider(
		otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs:      []string{ipp.configFile},
				Providers: map[string]confmap.Provider{fmp.Scheme(): fmp},
			},
		})
	if err != nil {
		return err
	}

	settings := otelcol.CollectorSettings{
		BuildInfo:             component.NewDefaultBuildInfo(),
		Factories:             ipp.factories,
		ConfigProvider:        configProvider,
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
		os.Remove(ipp.configFile)
	}
	ipp.wg.Wait()
	stopped = ipp.stopped
	return stopped, err
}

func (ipp *inProcessCollector) WatchResourceConsumption() error {
	return nil
}

func (ipp *inProcessCollector) GetProcessMon() *process.Process {
	return nil
}

func (ipp *inProcessCollector) GetTotalConsumption() *ResourceConsumption {
	return &ResourceConsumption{
		CPUPercentAvg: 0,
		CPUPercentMax: 0,
		RAMMiBAvg:     0,
		RAMMiBMax:     0,
	}
}

func (ipp *inProcessCollector) GetResourceConsumption() string {
	return ""
}
