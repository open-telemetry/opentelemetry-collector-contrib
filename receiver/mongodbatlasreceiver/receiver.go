// Copyright The OpenTelemetry Authors
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
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/pdata/pmetric"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

type receiver struct {
	log         *zap.Logger
	cfg         *Config
	client      *internal.MongoDBAtlasClient
	lastRun     time.Time
	mb          *metadata.MetricsBuilder
	stopperChan chan struct{}
}

type timeconstraints struct {
	start      string
	end        string
	resolution string
}

func newMongoDBAtlasReceiver(settings rcvr.CreateSettings, cfg *Config) *receiver {
	client := internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, cfg.RetrySettings, settings.Logger)
	return &receiver{
		log:         settings.Logger,
		cfg:         cfg,
		client:      client,
		mb:          metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		stopperChan: make(chan struct{}),
	}
}

func newMongoDBAtlasScraper(recv *receiver) (scraperhelper.Scraper, error) {
	return scraperhelper.NewScraper(typeStr, recv.scrape, scraperhelper.WithShutdown(recv.shutdown))
}

func (s *receiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := time.Now()
	if err := s.poll(ctx, s.timeConstraints(now)); err != nil {
		return pmetric.Metrics{}, err
	}
	s.lastRun = now
	return s.mb.Emit(), nil
}

func (s *receiver) timeConstraints(now time.Time) timeconstraints {
	var start time.Time
	if s.lastRun.IsZero() {
		start = now.Add(s.cfg.CollectionInterval * -1)
	} else {
		start = s.lastRun
	}
	return timeconstraints{
		start.UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		s.cfg.Granularity,
	}
}

func (s *receiver) shutdown(context.Context) error {
	return s.client.Shutdown()
}

func (s *receiver) poll(ctx context.Context, time timeconstraints) error {
	orgs, err := s.client.Organizations(ctx)
	if err != nil {
		return fmt.Errorf("error retrieving organizations: %w", err)
	}
	for _, org := range orgs {
		projects, err := s.client.Projects(ctx, org.ID)
		if err != nil {
			return fmt.Errorf("error retrieving projects: %w", err)
		}
		for _, project := range projects {
			nodeClusterMap, err := s.getNodeClusterNameMap(ctx, project.ID)
			if err != nil {
				return fmt.Errorf("error collecting clusters from project %s: %w", project.ID, err)
			}

			processes, err := s.client.Processes(ctx, project.ID)
			if err != nil {
				return fmt.Errorf("error retrieving MongoDB Atlas processes for project %s: %w", project.ID, err)
			}
			for _, process := range processes {
				clusterName := nodeClusterMap[process.UserAlias]

				if err := s.extractProcessMetrics(ctx, time, org.Name, project, process, clusterName); err != nil {
					return fmt.Errorf("error when polling process metrics from MongoDB Atlas for process %s: %w", process.ID, err)
				}

				if err := s.extractProcessDatabaseMetrics(ctx, time, org.Name, project, process, clusterName); err != nil {
					return fmt.Errorf("error when polling process database metrics from MongoDB Atlas for process %s: %w", process.ID, err)
				}

				if err := s.extractProcessDiskMetrics(ctx, time, org.Name, project, process, clusterName); err != nil {
					return fmt.Errorf("error when polling process disk metrics from MongoDB Atlas for process %s: %w", process.ID, err)
				}
			}
		}
	}
	return nil
}

func (s *receiver) getNodeClusterNameMap(
	ctx context.Context,
	projectID string,
) (map[string]string, error) {
	clusterMap := make(map[string]string)
	clusters, err := s.client.GetClusters(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		// URI in the form mongodb://host1.mongodb.net:27017,host2.mongodb.net:27017,host3.mongodb.net:27017
		nodes := strings.Split(strings.TrimPrefix(cluster.MongoURI, "mongodb://"), ",")
		for _, node := range nodes {
			// Remove the port from the node
			n, _, _ := strings.Cut(node, ":")
			clusterMap[n] = cluster.Name
		}
	}

	return clusterMap, nil
}

func (s *receiver) extractProcessMetrics(
	ctx context.Context,
	time timeconstraints,
	orgName string,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	clusterName string,
) error {
	if err := s.client.ProcessMetrics(
		ctx,
		s.mb,
		project.ID,
		process.Hostname,
		process.Port,
		time.start,
		time.end,
		time.resolution,
	); err != nil {
		return fmt.Errorf("error when polling process metrics from MongoDB Atlas: %w", err)
	}

	s.mb.EmitForResource(
		metadata.WithMongodbAtlasOrgName(orgName),
		metadata.WithMongodbAtlasProjectName(project.Name),
		metadata.WithMongodbAtlasProjectID(project.ID),
		metadata.WithMongodbAtlasHostName(process.Hostname),
		metadata.WithMongodbAtlasUserAlias(process.UserAlias),
		metadata.WithMongodbAtlasClusterName(clusterName),
		metadata.WithMongodbAtlasProcessPort(strconv.Itoa(process.Port)),
		metadata.WithMongodbAtlasProcessTypeName(process.TypeName),
		metadata.WithMongodbAtlasProcessID(process.ID),
	)

	return nil
}

func (s *receiver) extractProcessDatabaseMetrics(
	ctx context.Context,
	time timeconstraints,
	orgName string,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	clusterName string,
) error {
	processDatabases, err := s.client.ProcessDatabases(
		ctx,
		project.ID,
		process.Hostname,
		process.Port,
	)
	if err != nil {
		return fmt.Errorf("error retrieving process databases: %w", err)
	}

	for _, db := range processDatabases {
		if err := s.client.ProcessDatabaseMetrics(
			ctx,
			s.mb,
			project.ID,
			process.Hostname,
			process.Port,
			db.DatabaseName,
			time.start,
			time.end,
			time.resolution,
		); err != nil {
			return fmt.Errorf("error when polling database metrics from MongoDB Atlas: %w", err)
		}
		s.mb.EmitForResource(
			metadata.WithMongodbAtlasOrgName(orgName),
			metadata.WithMongodbAtlasProjectName(project.Name),
			metadata.WithMongodbAtlasProjectID(project.ID),
			metadata.WithMongodbAtlasHostName(process.Hostname),
			metadata.WithMongodbAtlasUserAlias(process.UserAlias),
			metadata.WithMongodbAtlasClusterName(clusterName),
			metadata.WithMongodbAtlasProcessPort(strconv.Itoa(process.Port)),
			metadata.WithMongodbAtlasProcessTypeName(process.TypeName),
			metadata.WithMongodbAtlasProcessID(process.ID),
			metadata.WithMongodbAtlasDbName(db.DatabaseName),
		)
	}
	return nil
}

func (s *receiver) extractProcessDiskMetrics(
	ctx context.Context,
	time timeconstraints,
	orgName string,
	project *mongodbatlas.Project,
	process *mongodbatlas.Process,
	clusterName string,
) error {
	for _, disk := range s.client.ProcessDisks(ctx, project.ID, process.Hostname, process.Port) {
		if err := s.client.ProcessDiskMetrics(
			ctx,
			s.mb,
			project.ID,
			process.Hostname,
			process.Port,
			disk.PartitionName,
			time.start,
			time.end,
			time.resolution,
		); err != nil {
			return fmt.Errorf("error when polling disk metrics from MongoDB Atlas: %w", err)
		}
		s.mb.EmitForResource(
			metadata.WithMongodbAtlasOrgName(orgName),
			metadata.WithMongodbAtlasProjectName(project.Name),
			metadata.WithMongodbAtlasProjectID(project.ID),
			metadata.WithMongodbAtlasHostName(process.Hostname),
			metadata.WithMongodbAtlasUserAlias(process.UserAlias),
			metadata.WithMongodbAtlasClusterName(clusterName),
			metadata.WithMongodbAtlasProcessPort(strconv.Itoa(process.Port)),
			metadata.WithMongodbAtlasProcessTypeName(process.TypeName),
			metadata.WithMongodbAtlasProcessID(process.ID),
			metadata.WithMongodbAtlasDiskPartition(disk.PartitionName),
		)
	}
	return nil
}
