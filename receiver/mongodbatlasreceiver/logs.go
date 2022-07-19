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

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
)

// combindedLogsReceiver wraps alerts and log receivers in a single log receiver to be consumed by the factory
type combindedLogsReceiver struct {
	alerts *alertsReceiver
	logs   *receiver
}

func (c *combindedLogsReceiver) Start(ctx context.Context, host component.Host) error {

	var errs error

	// If we have an alerts receiver start alerts collection
	if c.alerts != nil {
		if err := c.alerts.Start(ctx, host); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	// If we have a logging receiver start log collection
	if c.logs != nil {
		if err := c.logs.Start(ctx, host); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func (c *combindedLogsReceiver) Shutdown(ctx context.Context) error {
	var errs error

	// If we have an alerts receiver shutdown alerts collection
	if c.alerts != nil {
		if err := c.alerts.Shutdown(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	// If we have a logging receiver shutdown log collection
	if c.logs != nil {
		if err := c.logs.Shutdown(ctx); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func (s *receiver) createProjectMap() (map[string]*Project, error) {
	// create a map of Projects
	projects := make(map[string]*Project)

	for _, project := range s.cfg.Logs.Projects {
		projects[project.Name] = project
	}

	return projects, nil
}

// function parses out the hostname from the specified cluster host
func FilterHostName(s string) []string {
	var hostnames []string

	// first check to make sure string is of adequate size
	if len(s) > 0 {
		// create an array with a comma delimiter from the original string
		tmp := strings.Split(s, ",")
		for _, t := range tmp {

			u, err := url.Parse(t)
			if err != nil {
				fmt.Printf("Error parsing out %s", t)
			}

			// separate hostname from scheme and port
			host, _, err := net.SplitHostPort(u.Host)
			if err != nil {
				// the scheme prefix was not added on to this string
				// thus the hostname will have been placed under the "Scheme" field
				hostnames = append(hostnames, u.Scheme)
			} else {
				// hostname parsed successfully
				hostnames = append(hostnames, host)
			}

		}
	}

	return hostnames
}

func (s *receiver) GetProjectInfo(ctx context.Context) {
	cfgProjects, err := s.createProjectMap()
	if err != nil {
		s.log.Debug("error ")
	}

	for {
		orgs, err := s.client.Organizations(ctx)
		if err != nil {
			s.log.Debug("error retrieving organizations: %w")
		}
		for _, org := range orgs {
			projects, err := s.client.Projects(ctx, org.ID)
			if err != nil {
				s.log.Debug("error retrieving projects: %w")
			}

			var filteredProjects map[string]*mongodbatlas.Project
			for _, project := range projects {
				if _, ok := cfgProjects[project.Name]; ok {
					filteredProjects[project.Name] = project
				}
			}

			for _, project := range filteredProjects {
				include, exclude := cfgProjects[project.Name].IncludeClusters, cfgProjects[project.Name].ExcludeClusters
				clusters, err := s.client.GetClusters(ctx, project.ID, include, exclude)
				if err != nil {
					s.log.Debug("error retreiving clusters from project")
				}

				s.collectClusterLogs(clusters, cfgProjects, *project)
			}
		}
	}
}

func (s *receiver) collectClusterLogs(clusters []mongodbatlas.Cluster, cfgProjects map[string]*Project, project mongodbatlas.Project) {
	for _, cluster := range clusters {
		hostnames := FilterHostName(cluster.ConnectionStrings.Standard)
		for _, hostname := range hostnames {
			var logs []model.LogEntry
			logs = s.appendLogs(logs, project.ID, hostname, "monogodb.gz")
			logs = s.appendLogs(logs, project.ID, hostname, "monogos.gz")

			if cfgProjects[project.Name].EnableAuditLogs {
				s.appendLogs(logs, project.ID, hostname, "mongodb-audit-log.gz")
				s.appendLogs(logs, project.ID, hostname, "mongos-audit-log.gz")
			}

		}
	}
}
