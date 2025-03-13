// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"

	faro "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type faroExporter struct {
	config    *Config
	client    *http.Client
	logger    *zap.Logger
	settings  component.TelemetrySettings
	userAgent string
}

func newExporter(cfg component.Config, set exporter.Settings) (*faroExporter, error) {
	oCfg := cfg.(*Config)

	if oCfg.Endpoint != "" {
		_, err := url.Parse(oCfg.Endpoint)
		if err != nil {
			return nil, errors.New("endpoint must be a valid URL")
		}
	}

	userAgent := fmt.Sprintf("%s/%s (%s/%s)",
		set.BuildInfo.Description, set.BuildInfo.Version, runtime.GOOS, runtime.GOARCH)

	return &faroExporter{
		config:    oCfg,
		logger:    set.Logger,
		userAgent: userAgent,
		settings:  set.TelemetrySettings,
	}, nil
}

func (fe *faroExporter) start(ctx context.Context, host component.Host) error {
	return nil
}

func (fe *faroExporter) export(ctx context.Context, fp []*faro.Payload) error {
	return nil
}

func (fe *faroExporter) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func (fe *faroExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}

func (fe *faroExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
