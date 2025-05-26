// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/integrationtest"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"golang.org/x/sync/errgroup"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// createConfigYaml creates a yaml config for an otel collector for testing.
func createConfigYaml(
	tb testing.TB,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	processors map[string]string,
	extensions map[string]string,
	pipelineType string,
	debug bool,
) string {
	tb.Helper()

	processorSection, processorList := createConfigSection(processors)
	extensionSection, extensionList := createConfigSection(extensions)
	exporters := []string{receiver.ProtocolName()}
	debugVerbosity := "basic"
	logLevel := "INFO"
	if debug {
		exporters = append(exporters, "debug")
		logLevel = "DEBUG"
	}

	format := `
receivers:%v
exporters:%v
  debug:
    verbosity: %s

processors:
  %s

extensions:
  %s

service:
  telemetry:
    metrics:
      address: 127.0.0.1:%d
    logs:
      level: %s
      sampling:
        enabled: false
  extensions: [%s]
  pipelines:
    %s:
      receivers: [%v]
      processors: [%s]
      exporters: [%s]
`

	return fmt.Sprintf(
		format,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		debugVerbosity,
		processorSection,
		extensionSection,
		testutil.GetAvailablePort(tb),
		logLevel,
		extensionList,
		pipelineType,
		sender.ProtocolName(),
		processorList,
		strings.Join(exporters, ","),
	)
}

func createConfigSection(m map[string]string) (sections, list string) {
	if len(m) > 0 {
		first := true
		for name, cfg := range m {
			sections += cfg + "\n"
			if !first {
				list += ","
			}
			list += name
			first = false
		}
	}
	return
}

// recreatableOtelCol creates an otel collector that can be used to simulate
// a crash of the collector. It implements the testbed.OtelColRunner interface.
type recreatableOtelCol struct {
	tempDir   string
	factories otelcol.Factories
	settings  otelcol.CollectorSettings
	configStr string
	errGrp    errgroup.Group
	cancel    context.CancelFunc

	mu  sync.Mutex
	col *otelcol.Collector
}

func newRecreatableOtelCol(tb testing.TB) *recreatableOtelCol {
	var (
		err       error
		factories otelcol.Factories
	)
	factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		otlpreceiver.NewFactory(),
	)
	require.NoError(tb, err)
	factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](
		filestorage.NewFactory(),
	)
	require.NoError(tb, err)
	factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory]()
	require.NoError(tb, err)
	factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		elasticsearchexporter.NewFactory(),
		debugexporter.NewFactory(),
	)
	require.NoError(tb, err)
	return &recreatableOtelCol{
		tempDir:   tb.TempDir(),
		factories: factories,
	}
}

func (c *recreatableOtelCol) PrepareConfig(_ *testing.T, configStr string) (func(), error) {
	configCleanup := func() {
		// NoOp
	}
	c.configStr = configStr
	return configCleanup, nil
}

func (c *recreatableOtelCol) Start(_ testbed.StartParams) error {
	var err error

	confFile, err := os.CreateTemp(c.tempDir, "conf-")
	if err != nil {
		return err
	}

	if _, err = confFile.Write([]byte(c.configStr)); err != nil {
		os.Remove(confFile.Name())
		return err
	}

	cfgProviderSettings := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:              []string{confFile.Name()},
			ProviderFactories: []confmap.ProviderFactory{fileprovider.NewFactory()},
		},
	}

	c.settings = otelcol.CollectorSettings{
		BuildInfo:              component.NewDefaultBuildInfo(),
		Factories:              func() (otelcol.Factories, error) { return c.factories, nil },
		ConfigProviderSettings: cfgProviderSettings,
		SkipSettingGRPCLogger:  true,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.col, err = otelcol.NewCollector(c.settings)
	if err != nil {
		return err
	}

	return c.run()
}

func (c *recreatableOtelCol) Stop() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.col != nil {
		c.col.Shutdown()
		c.col = nil
	}

	if err := c.errGrp.Wait(); err != nil {
		return false, err
	}
	return true, nil
}

func (c *recreatableOtelCol) Restart(graceful bool, shutdownFor time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.col == nil {
		return nil
	}

	c.col.Shutdown()
	if !graceful {
		c.cancel()
	}
	err := c.errGrp.Wait()
	if err != nil {
		return fmt.Errorf("failed to stop old collector: %w", err)
	}

	c.col, err = otelcol.NewCollector(c.settings)
	if err != nil {
		return fmt.Errorf("failed to create new collector: %w", err)
	}

	time.Sleep(shutdownFor)
	return c.run()
}

func (c *recreatableOtelCol) WatchResourceConsumption() error {
	return nil
}

func (c *recreatableOtelCol) GetProcessMon() *process.Process {
	return nil
}

func (c *recreatableOtelCol) GetTotalConsumption() *testbed.ResourceConsumption {
	return &testbed.ResourceConsumption{
		CPUPercentAvg: 0,
		CPUPercentMax: 0,
		RAMMiBAvg:     0,
		RAMMiBMax:     0,
	}
}

func (c *recreatableOtelCol) GetResourceConsumption() string {
	return ""
}

func (c *recreatableOtelCol) run() error {
	var ctx context.Context
	ctx, c.cancel = context.WithCancel(context.Background())

	col := c.col
	c.errGrp.Go(func() error {
		// Ignore context canceled errors
		if err := col.Run(ctx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})

	for {
		switch state := col.GetState(); state {
		case otelcol.StateStarting:
			time.Sleep(time.Second)
		case otelcol.StateRunning:
			return nil
		default:
			return fmt.Errorf("unable to start, otelcol state is %d", state)
		}
	}
}
