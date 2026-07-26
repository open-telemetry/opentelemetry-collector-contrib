// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	oCfg := cfg.(*Config)
	rcvr := newOsQueryReceiver(oCfg, set)

	changeOnlyFactory := scraper.NewFactory(metadata.Type, nil,
		scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
			return scraper.NewLogs(rcvr.collect, scraper.WithStart(rcvr.start), scraper.WithShutdown(rcvr.shutdown))
		}, metadata.LogsStability))
	changeOnly, err := scraperhelper.NewLogsController(
		&oCfg.ControllerConfig, set, nextConsumer,
		scraperhelper.AddFactoryWithConfig(changeOnlyFactory, cfg),
	)
	if err != nil {
		return nil, err
	}

	if oCfg.SnapshotInterval <= 0 || len(oCfg.Collections) == 0 {
		return changeOnly, nil
	}

	snapshotFactory := scraper.NewFactory(metadata.Type, nil,
		scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
			return scraper.NewLogs(rcvr.snapshotCollect)
		}, metadata.LogsStability))
	snapshotControllerConfig := oCfg.ControllerConfig
	snapshotControllerConfig.CollectionInterval = oCfg.SnapshotInterval
	snapshot, err := scraperhelper.NewLogsController(
		&snapshotControllerConfig, set, nextConsumer,
		scraperhelper.AddFactoryWithConfig(snapshotFactory, cfg),
	)
	if err != nil {
		return nil, err
	}

	return &dualIntervalReceiver{changeOnly: changeOnly, snapshot: snapshot}, nil
}

// dualIntervalReceiver composes two independently-ticking receiver.Logs: a
// change-only scrape driven by ControllerConfig.CollectionInterval, and a
// full-snapshot scrape driven by Config.SnapshotInterval. scraperhelper only
// supports one interval per controller, so a second interval needs a second
// controller. changeOnly is started first so the storage client it resolves
// (via rcvr.start) is ready before snapshot's first synchronous scrape fires.
type dualIntervalReceiver struct {
	changeOnly receiver.Logs
	snapshot   receiver.Logs
}

func (r *dualIntervalReceiver) Start(ctx context.Context, host component.Host) error {
	if err := r.changeOnly.Start(ctx, host); err != nil {
		return err
	}
	return r.snapshot.Start(ctx, host)
}

func (r *dualIntervalReceiver) Shutdown(ctx context.Context) error {
	return errors.Join(r.changeOnly.Shutdown(ctx), r.snapshot.Shutdown(ctx))
}
