// Copyright  The OpenTelemetry Authors
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

	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/collector/component"
)

// combindedLogsReceiver wraps alerts and log receivers in a single log receiver to be consumed by the factory
type combindedLogsReceiver struct {
	alerts *alertsReceiver
	logs   *logsReceiver
}

// Starts up the combined MongoDB Atlas Logs and Alert Receiver
func (c *combindedLogsReceiver) Start(ctx context.Context, host component.Host) error {
	var errs error

	if c.alerts != nil {
		if err := c.alerts.Start(ctx, host); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if c.logs != nil {
		if err := c.logs.Start(ctx, host); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// Shutsdown the combined MongoDB Atlas Logs and Alert Receiver
func (c *combindedLogsReceiver) Shutdown(ctx context.Context) error {
	var errs error

	if c.alerts != nil {
		if err := c.alerts.Shutdown(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if c.logs != nil {
		if err := c.logs.Shutdown(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}
