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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Runs intermittently, fetching info from SAP HANA, creating metrics/datapoints,
// and feeding them to a metricsConsumer.
type sapHanaScraper struct {
	settings component.ReceiverCreateSettings
	//mb       *metadata.MetricsBuilder
	uptime time.Duration
}

func newSapHanaScraper(settings component.ReceiverCreateSettings, cfg *Config) (scraperhelper.Scraper, error) {
	rs := &sapHanaScraper{
		settings: settings,
		//mb:       metadata.NewMetricsBuilder(cfg.Metrics),
	}
	return scraperhelper.NewScraper(typeStr, rs.Scrape)
}

// Scrape is called periodically, querying SAP HANA and building Metrics to send to
// the next consumer.
func (rs *sapHanaScraper) Scrape(context.Context) (pdata.Metrics, error) {
	return pdata.Metrics{}, nil
}
