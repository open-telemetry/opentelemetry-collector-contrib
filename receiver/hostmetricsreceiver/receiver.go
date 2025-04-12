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

	metadataPkg "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

const entityType = "host"

type hostEntitiesReceiver struct {
	cfg *Config

	nextLogs consumer.Logs
	cancel   context.CancelFunc

	settings *receiver.Settings
}

func (hmr *hostEntitiesReceiver) Start(ctx context.Context, _ component.Host) error {
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

	return nil
}

func (hmr *hostEntitiesReceiver) Shutdown(_ context.Context) error {
	if hmr.cancel != nil {
		hmr.cancel()
	}
	return nil
}

func (hmr *hostEntitiesReceiver) sendEntityEvent(ctx context.Context) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	out := metadataPkg.NewEntityEventsSlice()
	entityEvent := out.AppendEmpty()
	entityEvent.SetTimestamp(timestamp)
	state := entityEvent.SetEntityState()
	state.SetEntityType(entityType)

	if hmr.cfg.MetadataCollectionInterval != 0 {
		state.SetInterval(hmr.cfg.MetadataCollectionInterval)
	}

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
