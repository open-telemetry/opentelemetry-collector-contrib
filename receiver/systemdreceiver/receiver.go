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

	ctx    context.Context
	cancel context.CancelFunc

	mb *metadata.MetricsBuilder

	units []string
}

func newSystemdReceiver(
	set receiver.Settings,
	logger *zap.Logger,
	cfg *Config,
) (receiver.Metrics, error) {
	r := &systemdReceiver{
		logger: logger,
		config: cfg,
		mb:     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, set),
	}
	return r, nil
}

func (s *systemdReceiver) Start(ctx context.Context, _ component.Host) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	return nil
}

func (s *systemdReceiver) Shutdown(_ context.Context) error {
	s.cancel()
	return nil
}
