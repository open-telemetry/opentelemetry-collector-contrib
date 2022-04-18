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
