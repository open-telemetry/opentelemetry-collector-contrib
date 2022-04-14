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

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var _ component.Receiver = (*vcenterReceiver)(nil)

type vcenterReceiver struct {
	config  *Config
	logger  *zap.Logger
	scraper component.Receiver
}

func (v *vcenterReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	if v.scraper != nil && v.config.MetricsConfig.Endpoint != "" {
		scraperErr := v.scraper.Start(ctx, host)
		if scraperErr != nil {
			// Start should not stop the collector if the metrics client connection attempt does not succeed,
			// so we log on start when we cannot connect
			v.logger.Error(fmt.Sprintf("unable to initially connect to vSphere SDK: %s", scraperErr.Error()))
		}
	}
	return err
}

func (v *vcenterReceiver) Shutdown(ctx context.Context) error {
	var err error
	if v.scraper != nil {
		scraperErr := v.scraper.Shutdown(ctx)
		if scraperErr != nil {
			err = multierr.Append(err, scraperErr)
		}
	}
	return err
}
