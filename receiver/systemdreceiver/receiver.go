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

type newClientFunc func(context.Context) (dbusClient, error)

type systemdReceiver struct {
	logger *zap.Logger
	config *Config

	client        dbusClient
	newClientFunc newClientFunc

	ctx context.Context

	mb *metadata.MetricsBuilder

	units []string
}

func newSystemdReceiver(
	settings receiver.Settings,
	cfg *Config,
	newClientFunc newClientFunc) *systemdReceiver {
	r := &systemdReceiver{
		logger:        settings.TelemetrySettings.Logger,
		config:        cfg,
		mb:            metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		newClientFunc: newClientFunc,
	}
	return r
}

func (s *systemdReceiver) Start(ctx context.Context, _ component.Host) error {
	s.ctx = ctx
	c, err := s.newClientFunc(s.ctx)
	if err != nil {
		return err
	}
	s.client = c
	return nil
}

func (s *systemdReceiver) Shutdown(_ context.Context) error {
	if s.client != nil {
		s.client.Close()
	}
	return nil
}
