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

//go:build windows
// +build windows

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)
	rcvr := newIisReceiver(params, cfg, nextConsumer)

	scraper, err := scraperhelper.NewScraper(typeStr, rcvr.scrape,
		scraperhelper.WithStart(rcvr.start),
		scraperhelper.WithShutdown(rcvr.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, nextConsumer,
		scraperhelper.AddScraper(scraper),
	)
}
