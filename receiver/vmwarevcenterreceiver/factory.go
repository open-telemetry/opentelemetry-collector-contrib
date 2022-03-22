// Copyright  OpenTelemetry Authors
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

package vmwarevcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver"

//go:generate mdatagen metadata.yaml

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver/internal/metadata"
)

const (
	typeStr = "vcenter"
)

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver),
		component.WithLogsReceiver(createLogsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)), CollectionInterval: time.Minute},
		TLSClientSetting:          configtls.TLSClientSetting{},
		Metrics:                   metadata.DefaultMetricsSettings(),
		Endpoint:                  "",
		Username:                  "",
		Password:                  "",
	}
}

func createLogsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Logs,
) (component.LogsReceiver, error) {
	return nil, nil
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)

	vcenterScraper := newVmwareVcenterScraper(params.Logger, cfg)
	scraper, err := scraperhelper.NewScraper(
		typeStr,
		vcenterScraper.scrape,
		scraperhelper.WithStart(vcenterScraper.start),
		scraperhelper.WithShutdown(vcenterScraper.shutdown),
	)

	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, consumer,
		scraperhelper.AddScraper(scraper),
	)
}
