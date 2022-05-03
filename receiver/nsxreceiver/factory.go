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
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
)

const typeStr = "nsx"

var errConfigNotNSX = errors.New("config was not a NSX receiver config")

type nsxReceiverFactory struct {
	receivers map[*Config]*nsxReceiver
}

// NewFactory creates a new receiver factory
func NewFactory() component.ReceiverFactory {
	f := &nsxReceiverFactory{
		receivers: make(map[*Config]*nsxReceiver, 1),
	}
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(f.createMetricsReceiver),
		component.WithLogsReceiver(f.createLogsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	syslogConfig := syslogreceiver.NewFactory().CreateDefaultConfig()
	sConf := syslogConfig.(*syslogreceiver.SysLogConfig)
	return &Config{
		MetricsConfig: &MetricsConfig{
			ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(typeStr),
			Settings:                  metadata.DefaultMetricsSettings(),
		},
		LoggingConfig: &LoggingConfig{
			SysLogConfig: sConf,
		},
	}
}

func (f *nsxReceiverFactory) ensureReceiver(params component.ReceiverCreateSettings, config config.Receiver) *nsxReceiver {
	receiver := f.receivers[config.(*Config)]
	if receiver != nil {
		return receiver
	}
	rconfig := config.(*Config)
	receiver = &nsxReceiver{
		logger: params.Logger,
		config: rconfig,
	}
	f.receivers[config.(*Config)] = receiver
	return receiver
}

func (f *nsxReceiverFactory) createMetricsReceiver(ctx context.Context, params component.ReceiverCreateSettings, rConf config.Receiver, consumer consumer.Metrics) (component.MetricsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotNSX
	}
	r := f.ensureReceiver(params, cfg)
	s := newScraper(cfg, params.TelemetrySettings)

	scraper, err := scraperhelper.NewScraper(
		typeStr,
		s.scrape,
		scraperhelper.WithStart(s.start),
	)
	if err != nil {
		return nil, err
	}

	scraperController, err := scraperhelper.NewScraperControllerReceiver(
		&cfg.MetricsConfig.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)

	r.scraper = scraperController
	return r, err
}

func (f *nsxReceiverFactory) createLogsReceiver(
	c context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Logs,
) (component.LogsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotNSX
	}
	rcvr := f.ensureReceiver(params, cfg)
	rcvr.logsReceiver = newLogsReceiver(cfg, params, consumer)
	return rcvr, nil
}
