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

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/atlas/mongodbatlas"
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

type timeconstraints struct {
	start      string
	end        string
	resolution string
}

func newMongoDBAtlasScraper(log *zap.Logger, cfg *Config) (scraperhelper.Scraper, error) {
	client, err := internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, cfg.RetrySettings, log)
	if err != nil {
		return nil, err
	}
	recv := &receiver{log: log, cfg: cfg, client: client}
	return scraperhelper.NewScraper(typeStr, recv.scrape, scraperhelper.WithShutdown(recv.shutdown))
}

func (s *receiver) scrape(ctx context.Context) (pdata.Metrics, error) {
	var start time.Time
	if s.lastRun.IsZero() {
		start = time.Now().Add(s.cfg.CollectionInterval * -1)
	} else {
		start = s.lastRun
	}
	now := time.Now()
	timeConstraints := timeconstraints{
		start.UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		s.cfg.Granularity,
	}
	metrics, err := s.poll(ctx, timeConstraints)
	if err != nil {
		return pdata.Metrics{}, err
	}
	s.lastRun = now
	return metrics, nil
}

func (s *receiver) shutdown(context.Context) error {
	return s.client.Shutdown()
}

func (s *receiver) poll(ctx context.Context, time timeconstraints) (pdata.Metrics, error) {
	resourceAttributes := pdata.NewAttributeMap()
	allMetrics := pdata.NewMetrics()
	orgs, err := s.client.Organizations(ctx)
	if err != nil {
		return pdata.Metrics{}, errors.Wrap(err, "error retrieving organizations")
	}
	for _, org := range orgs {
		resourceAttributes.InsertString("mongodb_atlas.org_name", org.Name)
		projects, err := s.client.Projects(ctx, org.ID)
		if err != nil {
			return pdata.Metrics{}, errors.Wrap(err, "error retrieving projects")
		}
		for _, project := range projects {
			resourceAttributes.InsertString("mongodb_atlas.project.name", project.Name)
			resourceAttributes.InsertString("mongodb_atlas.project.id", project.ID)
			processes, err := s.client.Processes(ctx, project.ID)
			if err != nil {
				return pdata.Metrics{}, errors.Wrap(err, "error retrieving MongoDB Atlas processes")
			}
			for _, process := range processes {
				resource := pdata.NewResource()
				resourceAttributes.CopyTo(resource.Attributes())
				resource.Attributes().InsertString("host.name", process.Hostname)
				resource.Attributes().InsertString("process.port", strconv.Itoa(process.Port))
				resource.Attributes().InsertString("process.type_name", process.TypeName)
				resource.Attributes().InsertString("process.id", process.ID)
				resourceMetrics, err := s.extractProcessMetrics(
					ctx,
					time,
					project,
					process,
					resource,
				)
				if err != nil {
					return pdata.Metrics{}, err
				}
				resourceMetrics.MoveAndAppendTo(allMetrics.ResourceMetrics())
			}
		}
	}
	return allMetrics, nil
}

func (s *receiver) extractProcessMetrics(
	ctx context.Context,
	time timeconstraints,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	resource pdata.Resource,
) (pdata.ResourceMetricsSlice, error) {
	processMetrics := pdata.NewResourceMetricsSlice()
	// This receiver will support both logs and metrics- if one pipeline
	//  or the other is not configured, it will be nil.
	metrics, err :=
		s.client.ProcessMetrics(
			ctx,
			resource,
			project.ID,
			process.Hostname,
			process.Port,
			time.start,
			time.end,
			time.resolution,
		)
	if err != nil {
		return pdata.ResourceMetricsSlice{}, errors.Wrap(
			err,
			"error when polling process metrics from MongoDB Atlas",
		)
	}
	metrics.ResourceMetrics().MoveAndAppendTo(processMetrics)

	databaseMetrics, err := s.extractProcessDatabaseMetrics(ctx, time, project, process, resource)
	if err != nil {
		return pdata.ResourceMetricsSlice{}, errors.Wrap(
			err,
			"error when polling process database metrics from MongoDB Atlas",
		)
	}
	databaseMetrics.MoveAndAppendTo(processMetrics)

	diskMetrics, err := s.extractProcessDiskMetrics(ctx, time, project, process, resource)
	if err != nil {
		return pdata.ResourceMetricsSlice{}, errors.Wrap(
			err,
			"error when polling process disk metrics from MongoDB Atlas",
		)
	}
	diskMetrics.MoveAndAppendTo(processMetrics)

	return processMetrics, nil
}

func (s *receiver) extractProcessDatabaseMetrics(
	ctx context.Context,
	time timeconstraints,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	resource pdata.Resource,
) (pdata.ResourceMetricsSlice, error) {
	pdMetrics := pdata.NewResourceMetricsSlice()
	processDatabases, err := s.client.ProcessDatabases(
		ctx,
		project.ID,
		process.Hostname,
		process.Port,
	)
	if err != nil {
		return pdata.ResourceMetricsSlice{}, errors.Wrap(err, "error retrieving process databases")
	}

	for _, db := range processDatabases {
		dbResource := pdata.NewResource()
		resource.CopyTo(dbResource)
		resource.Attributes().
			InsertString("mongodb_atlas.db.name", db.DatabaseName)
		metrics, err := s.client.ProcessDatabaseMetrics(
			ctx,
			resource,
			project.ID,
			process.Hostname,
			process.Port,
			db.DatabaseName,
			time.start,
			time.end,
			time.resolution,
		)
		if err != nil {
			return pdata.ResourceMetricsSlice{}, errors.Wrap(
				err,
				"error when polling database metrics from MongoDB Atlas",
			)
		}
		metrics.ResourceMetrics().MoveAndAppendTo(pdMetrics)
	}
	return pdMetrics, nil
}

func (s *receiver) extractProcessDiskMetrics(
	ctx context.Context,
	time timeconstraints,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	resource pdata.Resource,
) (pdata.ResourceMetricsSlice, error) {
	pdMetrics := pdata.NewResourceMetricsSlice()
	for _, disk := range s.client.ProcessDisks(ctx, project.ID, process.Hostname, process.Port) {
		diskResource := pdata.NewResource()
		resource.CopyTo(diskResource)
		diskResource.Attributes().
			InsertString("mongodb_atlas.disk.partition", disk.PartitionName)
		metrics, err := s.client.ProcessDiskMetrics(
			ctx,
			diskResource,
			project.ID,
			process.Hostname,
			process.Port,
			disk.PartitionName,
			time.start,
			time.end,
			time.resolution,
		)
		if err != nil {
			return pdata.ResourceMetricsSlice{}, errors.Wrap(
				err,
				"error when polling from MongoDB Atlas",
			)
		}
		metrics.ResourceMetrics().MoveAndAppendTo(pdMetrics)
	}
	return pdMetrics, nil
}
