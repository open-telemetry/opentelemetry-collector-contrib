// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

const (
	typeStr = "saphana"
)

// NewFactory creates a factory for SAP HANA receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	scs := scraperhelper.NewDefaultScraperControllerSettings(typeStr)
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "localhost:33015",
		},
		TLSClientSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		ScraperControllerSettings: scs,
		Metrics:                   metadata.DefaultMetricsSettings(),
	}
}

var errConfigNotSAPHANA = errors.New("config was not an sap hana receiver config")

func createMetricsReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotSAPHANA
	}
	scraper, err := newSapHanaScraper(set.TelemetrySettings, c)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&c.ScraperControllerSettings, set, consumer, scraperhelper.AddScraper(scraper))
}
