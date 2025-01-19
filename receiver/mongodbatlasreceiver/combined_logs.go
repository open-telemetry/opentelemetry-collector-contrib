// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

// combinedLogsReceiver wraps alerts and log receivers in a single log receiver to be consumed by the factory
type combinedLogsReceiver struct {
	alerts     *alertsReceiver
	logs       *logsReceiver
	events     *eventsReceiver
	accessLogs *accessLogsReceiver
	storageID  *component.ID
	id         component.ID
}

// Starts up the combined MongoDB Atlas Logs and Alert Receiver
func (c *combinedLogsReceiver) Start(ctx context.Context, host component.Host) error {
	var errs error

	storageClient, err := adapter.GetStorageClient(ctx, host, c.storageID, c.id)
	if err != nil {
		return fmt.Errorf("failed to get storage client: %w", err)
	}

	if c.alerts != nil {
		if err := c.alerts.Start(ctx, host, storageClient); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.logs != nil {
		if err := c.logs.Start(ctx, host); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.events != nil {
		if err := c.events.Start(ctx, host, storageClient); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.accessLogs != nil {
		if err := c.accessLogs.Start(ctx, host, storageClient); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}

// Shutsdown the combined MongoDB Atlas Logs and Alert Receiver
func (c *combinedLogsReceiver) Shutdown(ctx context.Context) error {
	var errs error

	if c.alerts != nil {
		if err := c.alerts.Shutdown(ctx); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.logs != nil {
		if err := c.logs.Shutdown(ctx); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.events != nil {
		if err := c.events.Shutdown(ctx); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if c.accessLogs != nil {
		if err := c.accessLogs.Shutdown(ctx); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}
