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
	"strconv"
	"time"

	"github.com/pkg/errors"
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
	metrics, err := s.poll(ctx, start.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), s.cfg.Granularity)
	if err != nil {
		return pdata.Metrics{}, err
	}
	s.lastRun = now
	return metrics, nil
}

func (s *receiver) poll(ctx context.Context, start string, end string, resolution string) (pdata.Metrics, error) {
	resourceAttributes := pdata.NewAttributeMap()
	allMetrics := pdata.NewMetrics()
	orgs, err := s.client.Organizations(ctx)
	if err != nil {
		return pdata.Metrics{}, errors.Wrap(err, "error retrieving organizations")
	}
	for _, org := range orgs {
		resourceAttributes.InsertString("mongodb.atlas.org_name", org.Name)
		projects, err := s.client.Projects(ctx, org.ID)
		if err != nil {
			return pdata.Metrics{}, errors.Wrap(err, "error retrieving projects")
		}
		for _, project := range projects {
			resourceAttributes.InsertString("mongodb.atlas.project", project.Name)
			processes, err := s.client.Processes(ctx, project.ID)
			if err != nil {
				return pdata.Metrics{}, errors.Wrap(err, "error retrieving MongoDB Atlas processes")
			}
			for _, process := range processes {
				resource := pdata.NewResource()
				resourceAttributes.CopyTo(resource.Attributes())
				resource.Attributes().InsertString("host.name", process.Hostname)
				resource.Attributes().InsertString("process.port", strconv.Itoa(process.Port))
				// This receiver will support both logs and metrics- if one pipeline
				//  or the other is not configured, it will be nil.
				metrics, err :=
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
				if err != nil {
					return pdata.Metrics{}, errors.Wrap(err, "error when polling process metrics from MongoDB Atlas")
				}
				metrics.ResourceMetrics().MoveAndAppendTo(allMetrics.ResourceMetrics())

				processDatabases, err := s.client.ProcessDatabases(
					ctx,
					project.ID,
					process.Hostname,
					process.Port,
				)
				if err != nil {
					return pdata.Metrics{}, errors.Wrap(err, "error retrieving process databases")
				}

				for _, db := range processDatabases {
					dbResource := pdata.NewResource()
					resource.CopyTo(dbResource)
					resource.Attributes().
						InsertString("mongodb.atlas.database_name", db.DatabaseName)
					metrics, err := s.client.ProcessDatabaseMetrics(
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
					if err != nil {
						return pdata.Metrics{}, errors.Wrap(err, "error when polling database metrics from MongoDB Atlas")
					}
					metrics.ResourceMetrics().MoveAndAppendTo(allMetrics.ResourceMetrics())
				}
				for _, disk := range s.client.ProcessDisks(ctx, project.ID, process.Hostname, process.Port) {
					diskResource := pdata.NewResource()
					resource.CopyTo(diskResource)
					diskResource.Attributes().
						InsertString("mongodb.atlas.partition", disk.PartitionName)
					metrics, err := s.client.ProcessDiskMetrics(
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
					if err != nil {
						return pdata.Metrics{}, errors.Wrap(err, "error when polling from MongoDB Atlas")
					}
					metrics.ResourceMetrics().MoveAndAppendTo(allMetrics.ResourceMetrics())
				}
			}
		}
	}
	return allMetrics, nil
}
