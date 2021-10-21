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

package mongodbatlasreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

type receiver struct {
	log     *zap.Logger
	cfg     *Config
	client  *internal.MongoDBAtlasClient
	lastRun time.Time
}

func newMongoDBAtlasScraper(log *zap.Logger, cfg *Config) (scraperhelper.Scraper, error) {
	client, err := internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, log)
	if err != nil {
		return nil, err
	}
	recv := &receiver{log: log, cfg: cfg, client: client}
	return scraperhelper.NewScraper(typeStr, recv.scrape)
}

func (s *receiver) scrape(ctx context.Context) (pdata.Metrics, error) {
	var start time.Time
	if s.lastRun.IsZero() {
		start = s.lastRun
	} else {
		start = time.Now().Add(s.cfg.CollectionInterval * -1)
	}
	now := time.Now()
	s.poll(ctx, start.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), s.cfg.Granularity)
	s.lastRun = now
	return pdata.Metrics{}, nil
}

func (s *receiver) poll(ctx context.Context, start string, end string, resolution string) { //nolint
	for _, org := range s.client.Organizations(ctx) {
		// Metrics collection starts here
		s.log.Debug("fetch resources for MongoDB Organization", zap.String("org", org.Name))
	}
}
