// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"
	"fmt"

	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"

	traces "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/traces"
)

type tracesExporter struct {
	config   *Config
	sender   *traces.Sender
	settings component.TelemetrySettings
	cancel   context.CancelFunc
}

// newTracesExporter creates new Logicmonitor Traces Exporter.
func newTracesExporter(_ context.Context, cfg component.Config, set exporter.Settings) *tracesExporter {
	oCfg := cfg.(*Config)

	// client construction is deferred to start
	return &tracesExporter{
		config:   oCfg,
		settings: set.TelemetrySettings,
	}
}

func (e *tracesExporter) start(ctx context.Context, host component.Host) error {
	client, err := e.config.ToClient(ctx, host, e.settings)
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	authParams := utils.AuthParams{
		AccessID:    e.config.APIToken.AccessID,
		AccessKey:   string(e.config.APIToken.AccessKey),
		BearerToken: string(e.config.Headers["Authorization"]),
	}

	ctx, e.cancel = context.WithCancel(ctx)
	e.sender, err = traces.NewSender(ctx, e.config.Endpoint, client, authParams, e.settings.Logger)
	if err != nil {
		return err
	}
	return nil
}

func (e *tracesExporter) PushTraceData(ctx context.Context, td ptrace.Traces) error {
	return e.sender.SendTraces(ctx, td)
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}

	return nil
}
