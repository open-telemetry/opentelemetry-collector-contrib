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
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// combindedLogsReceiver wraps alerts and log receivers in a single log receiver to be consumed by the factory
type combindedLogsReceiver struct {
	alerts *alertsReceiver
	logs   *receiver
}

type resourceInfo struct {
	Org      *mongodbatlas.Organization
	Project  *mongodbatlas.Project
	Cluster  *mongodbatlas.Cluster
	Hostname string
	LogName  string
}

// MongoDB Atlas Documentation reccommends a polling interval of 5  minutes: https://www.mongodb.com/docs/atlas/reference/api/logs/#logs
const collection_interval = time.Minute * 5

// Starts up the combined MongoDB Atlas Logs and Alert Receiver
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

// Shuts Down the Alerts and Log combined receiver
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

// Actual functionality the receiver executes from the Start function
func (s *receiver) KickoffReceiver(ctx context.Context) {
	stopper := make(chan struct{})
	s.stopperChanList = append(s.stopperChanList, stopper)
	cfgProjects, err := s.createProjectMap()
	if err != nil {
		s.log.Error("Failed to create project map", zap.Error(err))
	}

	for {
		// reterive all organizations listed in the MongoDB client
		orgs, err := s.client.Organizations(ctx)
		if err != nil {
			s.log.Error("Error retrieving organizations", zap.Error(err))
		}
		// get all projects listed for each of the organizations
		for _, org := range orgs {
			projects, err := s.client.Projects(ctx, org.ID)
			if err != nil {
				s.log.Error("Error retrieving projects", zap.Error(err))
			}

			// filter out any projects not specified in the config
			filteredProjects := make(map[string]*mongodbatlas.Project)
			for _, project := range projects {
				if _, ok := cfgProjects[project.Name]; ok {
					filteredProjects[project.Name] = project
				}
			}

			// get clusters for each of the projects
			for _, project := range filteredProjects {
				resource := resourceInfo{Org: org, Project: project}
				include, exclude := cfgProjects[project.Name].IncludeClusters, cfgProjects[project.Name].ExcludeClusters
				clusters, err := s.client.GetClusters(ctx, project.ID)
				if err != nil {
					s.log.Error("Failure to collect clusters from project: %w", zap.Error(err))
				}

				// check to include or exclude clusters
				switch {
				//keep all clusters if include and exclude are not specified
				case len(include) == 0 && len(exclude) == 0:
					break
				// include is initialized
				case len(include) > 0 && len(exclude) == 0:
					clusters, err = filterClusters(clusters, createStringMap(include), true)
					break
				// exclude is initialized
				case len(exclude) > 0 && len(include) == 0:
					clusters, err = filterClusters(clusters, createStringMap(exclude), false)
					break
				// both are initialized
				default:
					clusters = nil
					s.log.Error("Error can not have both include and exclude parameters initialized")
				}

				// collection interval loop,
				select {
				case <-ctx.Done():
					return
				case <-stopper:
					return
				case <-time.After(collection_interval):
					err = s.collectClusterLogs(clusters, cfgProjects, resource)
					if err != nil {
						s.log.Error("Failure to collect logs from cluster %w", zap.Error(err))
					}
				}
			}
		}
	}
}

func (s *receiver) collectClusterLogs(clusters []mongodbatlas.Cluster, cfgProjects map[string]*Project, r resourceInfo) error {
	for _, cluster := range clusters {
		hostnames := FilterHostName(cluster.ConnectionStrings.Standard)
		for _, hostname := range hostnames {
			r.Cluster = &cluster
			r.Hostname = hostname
			s.sendLogs(r, "mongodb.gz")
			s.sendLogs(r, "mongos.gz")

			if cfgProjects[r.Project.Name].EnableAuditLogs {
				s.sendAuditLogs(r, "mongodb-audit-log.gz")
				s.sendAuditLogs(r, "mongos-audit-log.gz")
			}
		}
	}

	return nil

}

func filterClusters(clusters []mongodbatlas.Cluster, keys map[string]string, include bool) ([]mongodbatlas.Cluster, error) {
	var filtered []mongodbatlas.Cluster
	for _, cluster := range clusters {
		if _, ok := keys[cluster.ID]; (!ok && !include) || (ok && include) {
			filtered = append(filtered, cluster)
		}
	}
	return filtered, nil
}

func createStringMap(in []string) map[string]string {
	list := make(map[string]string)
	for i := range in {
		list[in[i]] = in[i]
	}

	return list
}

func (s *receiver) getHostLogs(groupID, hostname, logName string) ([]model.LogEntry, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, collection_interval)
	if err != nil {
		return nil, err
	}
	// Pass this into a gzip reader for decoding
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}

	// Logs are in JSON format so create a JSON decoder to process them
	dec := json.NewDecoder(reader)

	entries := make([]model.LogEntry, 0)
	for {
		var entry model.LogEntry
		err := dec.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}
}

func (s *receiver) getHostAuditLogs(groupID, hostname, logName string) ([]model.AuditLog, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, collection_interval)
	if err != nil {
		return nil, err
	}
	// Pass this into a gzip reader for decoding
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}

	// Logs are in JSON format so create a JSON decoder to process them
	dec := json.NewDecoder(reader)

	entries := make([]model.AuditLog, 0)
	for {
		var entry model.AuditLog
		err := dec.Decode(&entry)
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}
}

func (s *receiver) sendLogs(r resourceInfo, logName string) {
	logs, err := s.getHostLogs(r.Project.ID, r.Hostname, logName)
	if err != nil && err != io.EOF {
		s.log.Warn("Failed to retreive logs", zap.Error(err))
	}

	for _, log := range logs {
		r.LogName = logName
		plog := mongodbEventToLogData(s.log, &log, r)
		s.consumer.ConsumeLogs(context.Background(), plog)
	}
}

func (s *receiver) sendAuditLogs(r resourceInfo, logName string) {
	logs, err := s.getHostAuditLogs(r.Project.ID, r.Hostname, logName)
	if err != nil && err != io.EOF {
		s.log.Warn("Failed to retreive logs", zap.Error(err))
	}

	for _, log := range logs {
		r.LogName = logName
		plog := mongodbAuditEventToLogData(s.log, &log, r)
		s.consumer.ConsumeLogs(context.Background(), plog)
	}
}
