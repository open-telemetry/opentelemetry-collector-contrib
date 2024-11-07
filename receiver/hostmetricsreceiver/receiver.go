// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pentity"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const entityType = "host"

type hostEntitiesReceiver struct {
	cfg *Config

	nextEntities consumer.Entities
	cancel       context.CancelFunc

	settings *receiver.Settings
}

func (hmr *hostEntitiesReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, hmr.cancel = context.WithCancel(ctx)
	if hmr.nextEntities != nil {
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

	out := pentity.NewEntities()
	entityEvent := out.ResourceEntities().AppendEmpty().ScopeEntities().AppendEmpty().EntityEvents().AppendEmpty()
	entityEvent.SetTimestamp(timestamp)
	entityEvent.SetEntityType(entityType)
	entityEvent.SetEmptyEntityState()

	err := hmr.nextEntities.ConsumeEntities(ctx, out)
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
