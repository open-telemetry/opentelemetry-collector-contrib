// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/log"
	"github.com/DataDog/datadog-agent/comp/core/log/logimpl"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/fx"
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

func newLogComponent(set component.TelemetrySettings, cfg *Config) (log.Component, error) {
	c, err := newConfigComponent(set, cfg)
	if err != nil {
		return nil, err
	}
	var l log.Component
	app := fx.New(
		logimpl.Module(),
		fx.Supply(c),
		fx.Populate(&l),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	return l, nil
}

func newAgentForwarder(set component.TelemetrySettings, cfg *Config) (coreconfig.AgentForwarder, error) {
	c, err := newConfigComponent(set, cfg)
	if err != nil {
		return nil, err
	}
	var f coreconfig.AgentForwarder
	app := fx.New(
		coreconfig.Module(),
		fx.Supply(c),
		fx.Populate(&f),
	)
	if err := app.Err(); err != nil {
		return nil, err
	}

	return f, nil
}
