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

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

var errConfigNotActiveDirectory = fmt.Errorf("config is not valid for the '%s' receiver", typeStr)

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotActiveDirectory
	}

	adds := newActiveDirectoryDSScraper(c.Metrics)
	scraper, err := scraperhelper.NewScraper(
		typeStr, 
		adds.scrape, 
		scraperhelper.WithStart(adds.start), 
		scraperhelper.WithShutdown(adds.shutdown),
	)

	if err != nil {
		return nil, err
	}
	
	return scraperhelper.NewScraperControllerReceiver(
		&c.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
