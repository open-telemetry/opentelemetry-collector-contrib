// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

const entityType = "host"

type hostMetricsReceiver struct {
	cfg *Config

	scraperHelper component.Component
	nextLogs      consumer.Logs
	cancel        context.CancelFunc

	settings *receiver.Settings
}

func (hmr *hostMetricsReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, hmr.cancel = context.WithCancel(ctx)
	if hmr.nextLogs != nil {
		hmr.sendEntityEvent(ctx)
		if hmr.cfg.MetadataCollectionInterval != 0 {
			ticker := time.NewTicker(hmr.cfg.MetadataCollectionInterval)
			go func() {
				for {
					select {
					case <-ticker.C:
						hmr.sendEntityEvent(ctx)
					case <-ctx.Done():
						ticker.Stop()
						return
					}
				}
			}()
		}
	}

	if hmr.scraperHelper != nil {
		return hmr.scraperHelper.Start(ctx, host)
	}
	return nil
}

func (hmr *hostMetricsReceiver) Shutdown(ctx context.Context) error {
	if hmr.cancel != nil {
		hmr.cancel()
	}
	if hmr.scraperHelper != nil {
		return hmr.scraperHelper.Shutdown(ctx)
	}
	return nil
}

func newHostMetricsReceiver(cfg *Config, set *receiver.Settings) *hostMetricsReceiver {
	r := &hostMetricsReceiver{
		cfg:      cfg,
		nextLogs: nil,
		settings: set,
	}

	return r
}

func (hmr *hostMetricsReceiver) sendEntityEvent(ctx context.Context) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	out := metadataPkg.NewEntityEventsSlice()
	entityEvent := out.AppendEmpty()
	entityEvent.SetTimestamp(timestamp)
	state := entityEvent.SetEntityState()
	state.SetEntityType(entityType)

	logs := out.ConvertAndMoveToLogs()

	err := hmr.nextLogs.ConsumeLogs(ctx, logs)
	if err != nil {
		hmr.settings.Logger.Error("Error sending entity event to the consumer", zap.Error(err))
	}

	// Note: receiver contract says that we need to retry sending if the
	// returned error is not Permanent. However, we are not doing it here.
	// Instead, we rely on the fact the metadata is collected periodically
	// and the entity events will be delivered on the next cycle. This is
	// fine because we deliver cumulative entity state.
	// This allows us to avoid stressing the Collector or its destination
	// unnecessarily (typically non-Permanent errors happen in stressed conditions).
}

// This is the map of already created hostmetric receivers for particular configurations.
// We maintain this map because the Factory is asked log and metric receivers separately
// when it gets CreateLogsReceiver() and CreateMetricsReceiver() but they must not
// create separate objects, they must use one receiver object per configuration.
var receivers = sharedcomponent.NewSharedComponents()
