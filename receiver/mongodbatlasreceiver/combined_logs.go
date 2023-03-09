// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	alerts    *alertsReceiver
	logs      *logsReceiver
	events    *eventsReceiver
	storageID *component.ID
	id        component.ID
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

	return errs
}
