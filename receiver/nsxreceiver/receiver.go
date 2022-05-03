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

package nsxreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.Receiver = (*nsxReceiver)(nil)

type nsxReceiver struct {
	config       *Config
	logger       *zap.Logger
	scraper      component.Receiver
	logsReceiver component.Receiver
}

func (n *nsxReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if n.scraper != nil && n.config.MetricsConfig.Endpoint != "" {
		if scraperErr := n.scraper.Start(ctx, host); scraperErr != nil {
			// Start should not stop the collector if the metrics client connection attempt does not succeed,
			// so we log on start when we cannot connect
			n.logger.Error(fmt.Sprintf("unable to initially start connecting to the NSX API: %s", scraperErr.Error()))
		}
	}

	if n.logsReceiver != nil {
		// if syslogreceiver is not bundled and logging is in the pipeline for NSX, we probably want to not start the collector
		if startErr := n.logsReceiver.Start(ctx, host); startErr != nil {
			err = multierr.Append(err, startErr)
		}
	}
	return err
}

// Shutdown calls appropriate shutdowns for the NSX receiver
func (n *nsxReceiver) Shutdown(ctx context.Context) error {
	var err error
	if n.scraper != nil {
		if scraperErr := n.scraper.Shutdown(ctx); scraperErr != nil {
			err = multierr.Append(err, scraperErr)
		}
	}
	if n.logsReceiver != nil {
		if logsErr := n.logsReceiver.Shutdown(ctx); logsErr != nil {
			err = multierr.Append(err, logsErr)
		}
	}
	return err
}
