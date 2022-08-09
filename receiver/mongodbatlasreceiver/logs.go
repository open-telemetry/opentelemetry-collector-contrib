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
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

type logsReceiver struct {
	log         *zap.Logger
	cfg         *Config
	client      *internal.MongoDBAtlasClient
	consumer    consumer.Logs
	stopperChan chan struct{}
	wg          sync.WaitGroup
}

type resourceInfo struct {
	Org      string
	Project  mongodbatlas.Project
	Cluster  mongodbatlas.Cluster
	Hostname string
	LogName  string
	start    string
	end      string
}

// MongoDB Atlas Documentation reccommends a polling interval of 5  minutes: https://www.mongodb.com/docs/atlas/reference/api/logs/#logs
const collectionInterval = time.Minute * 5

func newMongoDBAtlasLogsReceiver(settings component.ReceiverCreateSettings, cfg *Config) (*logsReceiver, error) {
	client, err := internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, cfg.RetrySettings, settings.Logger)
	if err != nil {
		return nil, err
	}
	recv := &logsReceiver{log: settings.Logger, cfg: cfg, client: client, stopperChan: make(chan struct{})}
	return recv, nil
}

// Log receiver logic
func (s *logsReceiver) Start(ctx context.Context, host component.Host) error {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.collect(ctx)
	}()
	return nil
}

func (s *logsReceiver) Shutdown(ctx context.Context) error {
	close(s.stopperChan)
	s.wg.Wait()
	return s.shutdown(ctx)
}

func (s *logsReceiver) shutdown(context.Context) error {
	return s.client.Shutdown()
}

// parseHostName parses out the hostname from the specified cluster host
func parseHostName(s string, logger *zap.Logger) []string {
	var hostnames []string

	// first check to make sure string is not empty
	if s == "" {
		return []string{}
	}

	// create an array with a comma delimiter from the original string
	tmp := strings.Split(s, ",")
	for _, t := range tmp {

		h := strings.TrimPrefix(t, "mongodb://")

		// separate hostname from scheme and port
		host, _, err := net.SplitHostPort(h)
		if err != nil {
			logger.Error("Could not parse out hostname: " + host)
			continue
		}
		// hostname parsed successfully
		hostnames = append(hostnames, host)
	}

	return hostnames
}

// KickoffReceiver spins off functionality of the receiver from the Start function
func (s *logsReceiver) collect(ctx context.Context) {
	resource := resourceInfo{start: strconv.Itoa(int(time.Now().Unix())), end: ""}
	for {
		// collection interval loop,
		select {
		case <-ctx.Done():
			return
		case <-s.stopperChan:
			return
		case <-time.After(collectionInterval):
		}

		for _, p := range s.cfg.Logs.Projects {
			project, err := s.client.GetOneProject(ctx, p.Name)
			if err != nil {
				s.log.Error("Error retrieving project", zap.Error(err))
			}

			org, err := s.client.GetOneOrganization(ctx, project.OrgID)
			if err != nil {
				s.log.Error("Error retrieving organization", zap.Error(err))
			}
			resource.Org = org.Name

			// get clusters for each of the projects
			resource.Project = *project
			clusters, err := s.processClusters(ctx, *p, &resource)
			if err != nil {
				s.log.Error("Failure to process Clusters", zap.Error(err))
			}

			if resource.end != "" {
				resource.start = resource.end
			}
			resource.end = strconv.Itoa(int(time.Now().Unix()))
			s.collectClusterLogs(clusters, p, resource)
		}
	}
}

func (s *logsReceiver) processClusters(ctx context.Context, project Project, r *resourceInfo) ([]mongodbatlas.Cluster, error) {
	include, exclude := project.IncludeClusters, project.ExcludeClusters
	clusters, err := s.client.GetClusters(ctx, r.Project.ID)
	if err != nil {
		s.log.Error("Failure to collect clusters from project: %w", zap.Error(err))
		return nil, err
	}

	// check to include or exclude clusters
	switch {
	// keep all clusters if include and exclude are not specified
	case len(include) == 0 && len(exclude) == 0:
		return clusters, nil
	// include is initialized
	case len(include) > 0 && len(exclude) == 0:
		return filterClusters(clusters, createStringSet(include), true), nil
	// exclude is initialized
	case len(exclude) > 0 && len(include) == 0:
		return filterClusters(clusters, createStringSet(exclude), false), nil
	// both are initialized
	default:
		return nil, errors.New("both Include and Exclude clusters configured")
	}
}

func (s *logsReceiver) collectClusterLogs(clusters []mongodbatlas.Cluster, project *Project, r resourceInfo) {
	for _, cluster := range clusters {
		hostnames := parseHostName(cluster.ConnectionStrings.Standard, s.log)
		for _, hostname := range hostnames {
			r.Cluster = cluster
			r.Hostname = hostname
			s.sendLogs(r, "mongodb.gz")
			s.sendLogs(r, "mongos.gz")

			if project.EnableAuditLogs {
				s.sendAuditLogs(r, "mongodb-audit-log.gz")
				s.sendAuditLogs(r, "mongos-audit-log.gz")
			}
		}
	}
}

func filterClusters(clusters []mongodbatlas.Cluster, keys map[string]struct{}, include bool) []mongodbatlas.Cluster {
	var filtered []mongodbatlas.Cluster
	for _, cluster := range clusters {
		if _, ok := keys[cluster.Name]; (!ok && !include) || (ok && include) {
			filtered = append(filtered, cluster)
		}
	}
	return filtered
}

func createStringSet(in []string) map[string]struct{} {
	list := map[string]struct{}{}
	for i := range in {
		list[in[i]] = struct{}{}
	}

	return list
}

func (s *logsReceiver) getHostLogs(groupID, hostname, logName, start, end string) ([]model.LogEntry, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, start, end)
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

func (s *logsReceiver) getHostAuditLogs(groupID, hostname, logName, start, end string) ([]model.AuditLog, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, start, end)
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

func (s *logsReceiver) sendLogs(r resourceInfo, logName string) {
	logs, err := s.getHostLogs(r.Project.ID, r.Hostname, logName, r.start, r.end)
	if err != nil && !errors.Is(err, io.EOF) {
		s.log.Warn("Failed to retrieve logs from: "+logName, zap.Error(err))
	}

	for _, log := range logs {
		r.LogName = logName
		plog := mongodbEventToLogData(s.log, log, r)
		err := s.consumer.ConsumeLogs(context.Background(), plog)
		if err != nil {
			s.log.Error("Failed to consume logs", zap.Error(err))
		}
	}
}

func (s *logsReceiver) sendAuditLogs(r resourceInfo, logName string) {
	logs, err := s.getHostAuditLogs(r.Project.ID, r.Hostname, logName, r.start, r.end)
	if err != nil && !errors.Is(err, io.EOF) {
		s.log.Warn("Failed to retrieve audit logs: "+logName, zap.Error(err))
	}

	for _, log := range logs {
		r.LogName = logName
		plog := mongodbAuditEventToLogData(s.log, log, r)
		err := s.consumer.ConsumeLogs(context.Background(), plog)
		if err != nil {
			s.log.Error("Failed to consume logs", zap.Error(err))
		}
	}
}
