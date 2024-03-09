// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

func newConfigComponent(set component.TelemetrySettings, cfg *Config) (coreconfig.Component, error) {
	var c coreconfig.Component
	yamldata := fmt.Sprintf(`logs_enabled: true
log_level: %s
site: %s
api_key: %s
apm_config:
  enabled: true
  apm_non_local_traffic: true
forwarder_timeout: 10`, set.Logger.Level().String(), cfg.API.Site, cfg.API.Key)

	tempDir, err := os.MkdirTemp("", "conf")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir) // Clean up

	// Create a temporary file within tempDir
	tempFilePath := filepath.Join(tempDir, "datadog.yaml")
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	// Write data to the temp file
	if _, err := io.WriteString(tempFile, yamldata); err != nil {
		return nil, err
	}
	app := fx.New(
		coreconfig.Module(),
		fx.Provide(func() coreconfig.Params {
			return coreconfig.NewAgentParams(tempFilePath)
		}),
		fx.Populate(&c),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	return c, nil
}

func newLogComponent(set component.TelemetrySettings) (log.Component, error) {
	return &zaplogger{
		logger: set.Logger,
	}, nil
}

func newAgentForwarder(set component.TelemetrySettings, cfg *Config, config coreconfig.Component) (defaultforwarder.Forwarder, error) {
	var f defaultforwarder.Component
	app := fx.New(
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
		defaultforwarder.Module(),
		fx.Supply(set.Logger),
		fx.Supply(cfg),
		fx.Supply(set),
		fx.Supply(config),
		fx.Provide(newLogComponent),

		fx.Provide(func(c coreconfig.Component, l log.Component) (defaultforwarder.Params, error) {
			return defaultforwarder.NewParams(c, l), nil
		}),
		fx.Populate(&f),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	return f, nil
}

type orchestratorinterfaceimpl struct {
	f defaultforwarder.Forwarder
}

func (o *orchestratorinterfaceimpl) Get() (defaultforwarder.Forwarder, bool) {
	return o.f, true
}

func (o *orchestratorinterfaceimpl) Reset() {
	o.f = nil
}

func newSerializer(set component.TelemetrySettings, cfg *Config) (*serializer.Serializer, error) {
	c, err := newConfigComponent(set, cfg)
	if err != nil {
		return nil, err
	}
	f, err := newAgentForwarder(set, cfg, c)
	if err != nil {
		return nil, err
	}
	return serializer.NewSerializer(f, &orchestratorinterfaceimpl{
		f: f,
	}, c, ""), nil
}

func newSerializerExporter(ctx context.Context, set exporter.CreateSettings, cfg *Config) (exporter.Metrics, error) {
	s, err := newSerializer(set.TelemetrySettings, cfg)
	if err != nil {
		return nil, err
	}
	factory := serializerexporter.NewFactory(s, nil, func(_ context.Context) (string, error) {
		return "", nil
	})
	if err != nil {
		return nil, err
	}
	exp, err := factory.CreateMetricsExporter(ctx, set, cfg)
	if err != nil {
		return nil, err
	}
	return exp, nil
}
