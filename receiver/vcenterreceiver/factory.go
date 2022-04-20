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

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

const (
	typeStr = "vcenter"
)

type vcenterReceiverFactory struct {
	receivers map[*Config]*vcenterReceiver
}

// NewFactory returns the receiver factory for the vcenterreceiver
func NewFactory() component.ReceiverFactory {
	f := &vcenterReceiverFactory{
		receivers: make(map[*Config]*vcenterReceiver),
	}
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(f.createMetricsReceiver),
	)
}

func createDefaultConfig() config.Receiver {
	return &Config{
		MetricsConfig: &MetricsConfig{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
				CollectionInterval: 2 * time.Minute,
			},
			TLSClientSetting: configtls.TLSClientSetting{},
			Metrics:          metadata.DefaultMetricsSettings(),
		},
	}
}

func (f *vcenterReceiverFactory) ensureReceiver(params component.ReceiverCreateSettings, config config.Receiver) *vcenterReceiver {
	receiver := f.receivers[config.(*Config)]
	if receiver != nil {
		return receiver
	}
	rconfig := config.(*Config)
	receiver = &vcenterReceiver{
		logger: params.Logger,
		config: rconfig,
	}
	f.receivers[config.(*Config)] = receiver
	return receiver
}

var errConfigNotVcenter = errors.New("config was not an vcenter receiver config")

func (f *vcenterReceiverFactory) createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotVcenter
	}
	r := f.ensureReceiver(params, cfg)
	vr := newVmwareVcenterScraper(params.Logger, cfg)
	scraper, err := scraperhelper.NewScraper(
		typeStr,
		vr.scrape,
		scraperhelper.WithStart(vr.Start),
		scraperhelper.WithShutdown(vr.Shutdown),
	)
	if err != nil {
		return nil, err
	}

	rcvr, err := scraperhelper.NewScraperControllerReceiver(
		&cfg.MetricsConfig.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
	if err != nil {
		return nil, err
	}
	r.scraper = rcvr
	return r, nil
}
