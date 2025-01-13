// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"
import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

type systemdReceiver struct {
	logger *zap.Logger
	config *Config

	client dbusClient

	ctx    context.Context
	cancel context.CancelFunc

	mb *metadata.MetricsBuilder

	units []string
}

func newSystemdReceiver(
	settings receiver.Settings,
	cfg *Config,
	client dbusClient) *systemdReceiver {
	r := &systemdReceiver{
		logger: settings.TelemetrySettings.Logger,
		config: cfg,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		client: client,
	}
	return r
}

func (s *systemdReceiver) Start(ctx context.Context, _ component.Host) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return nil
}

func (s *systemdReceiver) Shutdown(_ context.Context) error {
	s.client.Close()
	s.cancel()
	return nil
}
