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
		resourceAttributes.InsertString("org_name", org.Name)
		for _, project := range s.client.Projects(ctx, org.ID) {
			resourceAttributes.InsertString("project_name", project.Name)
			for _, process := range s.client.Processes(ctx, project.ID) {
				resource := pdata.NewResource()
				resourceAttributes.CopyTo(resource.Attributes())
				resource.Attributes().InsertString("hostname", process.Hostname)
				resource.Attributes().InsertString("port", strconv.Itoa(process.Port))
				if s.metricSink != nil {
					metrics, errs :=
						s.client.ProcessMetrics(
							ctx,
							resource,
							project.ID,
							process.Hostname,
							process.Port,
							start,
							end,
							resolution,
						)
					for _, err := range errs {
						s.log.Debug(
							"Encountered error when polling from MongoDB Atlas",
							zap.Error(err),
						)
					}
					s.metricSink.ConsumeMetrics(ctx, metrics)

					for _, db := range s.client.ProcessDatabases(ctx, project.ID, process.Hostname, process.Port) {
						dbResource := pdata.NewResource()
						resource.CopyTo(dbResource)
						resource.Attributes().InsertString("database_name", db.DatabaseName)
						metrics, errs := s.client.ProcessDatabaseMetrics(
							ctx,
							resource,
							project.ID,
							process.Hostname,
							process.Port,
							db.DatabaseName,
							start,
							end,
							resolution,
						)
						for _, err := range errs {
							s.log.Debug(
								"Encountered error when polling from MongoDB Atlas",
								zap.Error(err),
							)
						}
						s.metricSink.ConsumeMetrics(
							ctx,
							metrics,
						)
					}
					for _, disk := range s.client.ProcessDisks(ctx, project.ID, process.Hostname, process.Port) {
						diskResource := pdata.NewResource()
						resource.CopyTo(diskResource)
						diskResource.Attributes().InsertString("partition_name", disk.PartitionName)
						metrics, errs := s.client.ProcessDiskMetrics(
							ctx,
							diskResource,
							project.ID,
							process.Hostname,
							process.Port,
							disk.PartitionName,
							start,
							end,
							resolution,
						)
						for _, err := range errs {
							s.log.Debug(
								"Encountered error when polling from MongoDB Atlas",
								zap.Error(err),
							)
						}
						s.metricSink.ConsumeMetrics(
							ctx,
							metrics,
						)
					}
				} else {
					s.log.Debug("Metrics not enabled")
				}
			}
		}
		// }
	}
}
