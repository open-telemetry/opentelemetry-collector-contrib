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

package chronyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
)

const (
	typeStr = "chrony"

	// The stability level of the receiver.
	stability = component.StabilityLevelAlpha
)

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		newDefaultCongfig,
		component.WithMetricsReceiver(newMetricsReceiver, stability),
	)
}

func newMetricsReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	rCfg config.Receiver,
	consumer consumer.Metrics) (component.MetricsReceiver, error) {
	cfg, ok := rCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("wrong config provided: %w", errInvalidValue)
	}

	chronyc, err := chrony.New(cfg.Endpoint, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	scraper, err := scraperhelper.NewScraper(
		typeStr,
		newScraper(ctx, chronyc, cfg, set).scrape,
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		set,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
