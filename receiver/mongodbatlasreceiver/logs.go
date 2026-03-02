// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

const mongoDBMajorVersion4_2 = "4.2"

type logsReceiver struct {
	log         *zap.Logger
	cfg         *Config
	client      *internal.MongoDBAtlasClient
	consumer    consumer.Logs
	stopperChan chan struct{}
	wg          sync.WaitGroup
	start       time.Time
	end         time.Time
}

type projectContext struct {
	Project mongodbatlas.Project
	orgName string
}

// MongoDB Atlas Documentation recommends a polling interval of 5 minutes: https://www.mongodb.com/docs/atlas/reference/api/logs/#logs
const collectionInterval = time.Minute * 5

func newMongoDBAtlasLogsReceiver(settings rcvr.Settings, cfg *Config, consumer consumer.Logs) (*logsReceiver, error) {
	client, err := internal.NewMongoDBAtlasClient(cfg.BaseURL, cfg.PublicKey, string(cfg.PrivateKey), cfg.BackOffConfig, settings.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create MongoDB Atlas client for logs receiver: %w", err)
	}

	for _, p := range cfg.Logs.Projects {
		p.populateIncludesAndExcludes()
	}

	return &logsReceiver{
		log:         settings.Logger,
		cfg:         cfg,
		client:      client,
		stopperChan: make(chan struct{}),
		consumer:    consumer,
	}, nil
}

// Log receiver logic
func (s *logsReceiver) Start(ctx context.Context, _ component.Host) error {
	s.wg.Go(func() {
		s.start = time.Now().Add(-collectionInterval)
		s.end = time.Now()
		for {
			s.collect(ctx)
			// collection interval loop,
			select {
			case <-ctx.Done():
				return
			case <-s.stopperChan:
				return
			case <-time.After(collectionInterval):
				s.start = s.end
				s.end = time.Now()
			}
		}
	})
	return nil
}

func (s *logsReceiver) Shutdown(_ context.Context) error {
	close(s.stopperChan)
	s.wg.Wait()
	return s.client.Shutdown()
}

// parseHostNames parses out the hostname from the specified cluster host
func parseHostNames(s string, logger *zap.Logger) []string {
	var hostnames []string

	if s == "" {
		return []string{}
	}

	for t := range strings.SplitSeq(s, ",") {
		// separate hostname from scheme and port
		host, _, err := net.SplitHostPort(strings.TrimPrefix(t, "mongodb://"))
		if err != nil {
			logger.Error("Could not parse out hostname: " + host)
			continue
		}
		hostnames = append(hostnames, host)
	}

	return hostnames
}

// collect spins off functionality of the receiver from the Start function
func (s *logsReceiver) collect(ctx context.Context) {
	for _, projectCfg := range s.cfg.Logs.Projects {
		project, err := s.client.GetProject(ctx, projectCfg.Name)
		if err != nil {
			s.log.Error("Error retrieving project "+projectCfg.Name+":", zap.Error(err))
			continue
		}
		pc := projectContext{Project: *project}

		org, err := s.client.GetOrganization(ctx, project.OrgID)
		if err != nil {
			s.log.Error("Error retrieving organization", zap.Error(err))
			pc.orgName = "unknown"
		} else {
			pc.orgName = org.Name
		}

		// get clusters for each of the projects
		clusters, err := s.processClusters(ctx, *projectCfg, project.ID)
		if err != nil {
			s.log.Error("Failure to process Clusters", zap.Error(err))
		}

		s.collectClusterLogs(clusters, *projectCfg, pc)
	}
}

func (s *logsReceiver) processClusters(ctx context.Context, projectCfg LogsProjectConfig, projectID string) ([]mongodbatlas.Cluster, error) {
	clusters, err := s.client.GetClusters(ctx, projectID)
	if err != nil {
		s.log.Error("Failure to collect clusters from project: %w", zap.Error(err))
		return nil, err
	}

	return filterClusters(clusters, projectCfg.ProjectConfig)
}

type clusterInfo struct {
	ClusterName         string
	RegionName          string
	ProviderName        string
	MongoDBMajorVersion string
}

func (s *logsReceiver) collectClusterLogs(clusters []mongodbatlas.Cluster, projectCfg LogsProjectConfig, pc projectContext) {
	for i := range clusters {
		cluster := &clusters[i]
		c := clusterInfo{
			ClusterName:         cluster.Name,
			RegionName:          cluster.ProviderSettings.RegionName,
			ProviderName:        cluster.ProviderSettings.ProviderName,
			MongoDBMajorVersion: cluster.MongoDBMajorVersion,
		}

		hostnames := parseHostNames(cluster.ConnectionStrings.Standard, s.log)
		for _, hostname := range hostnames {
			// Defaults to true if not specified
			if projectCfg.EnableHostLogs == nil || *projectCfg.EnableHostLogs {
				s.log.Debug("Collecting logs for host", zap.String("hostname", hostname), zap.String("cluster", cluster.Name))
				s.collectLogs(pc, hostname, "mongodb.gz", c)
				s.collectLogs(pc, hostname, "mongos.gz", c)
			}

			// Defaults to false if not specified
			if projectCfg.EnableAuditLogs {
				s.log.Debug("Collecting audit logs for host", zap.String("hostname", hostname), zap.String("cluster", cluster.Name))
				s.collectAuditLogs(pc, hostname, "mongodb-audit-log.gz", c)
				s.collectAuditLogs(pc, hostname, "mongos-audit-log.gz", c)
			}
		}
	}
}

func filterClusters(clusters []mongodbatlas.Cluster, projectCfg ProjectConfig) ([]mongodbatlas.Cluster, error) {
	include, exclude := projectCfg.IncludeClusters, projectCfg.ExcludeClusters
	var allowed bool
	var clusterNameSet map[string]struct{}
	// check to include or exclude clusters
	switch {
	// keep all clusters if include and exclude are not specified
	case len(include) == 0 && len(exclude) == 0:
		return clusters, nil
	// include is initialized
	case len(include) > 0 && len(exclude) == 0:
		allowed = true
		clusterNameSet = projectCfg.includesByClusterName
	// exclude is initialized
	case len(exclude) > 0 && len(include) == 0:
		allowed = false
		clusterNameSet = projectCfg.excludesByClusterName
	// both are initialized
	default:
		return nil, errors.New("both Include and Exclude clusters configured")
	}

	var filtered []mongodbatlas.Cluster
	for i := range clusters {
		cluster := clusters[i]
		if _, ok := clusterNameSet[cluster.Name]; (!ok && !allowed) || (ok && allowed) {
			filtered = append(filtered, cluster)
		}
	}
	return filtered, nil
}

func (s *logsReceiver) getHostLogs(groupID, hostname, logName, clusterMajorVersion string) ([]model.LogEntry, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, s.start, s.end)
	if err != nil {
		return nil, err
	}

	return decodeLogs(s.log, clusterMajorVersion, buf)
}

func (s *logsReceiver) getHostAuditLogs(groupID, hostname, logName string) ([]model.AuditLog, error) {
	// Get gzip bytes buffer from API
	buf, err := s.client.GetLogs(context.Background(), groupID, hostname, logName, s.start, s.end)
	if err != nil {
		return nil, err
	}

	return decodeAuditJSON(s.log, buf)
}

func (s *logsReceiver) collectLogs(pc projectContext, hostname, logName string, clusterInfo clusterInfo) {
	logs, err := s.getHostLogs(pc.Project.ID, hostname, logName, clusterInfo.MongoDBMajorVersion)
	if err != nil && !errors.Is(err, io.EOF) {
		s.log.Warn("Failed to retrieve host logs", zap.Error(err), zap.String("hostname", hostname), zap.String("log", logName), zap.Time("startTime", s.start), zap.Time("endTime", s.end))
		return
	}

	if len(logs) == 0 {
		s.log.Warn("Attempted to retrieve host logs but received 0 logs", zap.Error(err), zap.String("log", logName), zap.String("hostname", hostname), zap.Time("startTime", s.start), zap.Time("endTime", s.end))
		return
	}

	plog := mongodbEventToLogData(s.log,
		logs,
		pc,
		hostname,
		logName,
		clusterInfo)
	err = s.consumer.ConsumeLogs(context.Background(), plog)
	if err != nil {
		s.log.Error("Failed to consume logs", zap.Error(err))
	}
}

func (s *logsReceiver) collectAuditLogs(pc projectContext, hostname, logName string, clusterInfo clusterInfo) {
	logs, err := s.getHostAuditLogs(
		pc.Project.ID,
		hostname,
		logName,
	)

	if err != nil && !errors.Is(err, io.EOF) {
		s.log.Warn("Failed to retrieve audit logs", zap.Error(err), zap.String("hostname", hostname), zap.String("log", logName), zap.Time("startTime", s.start), zap.Time("endTime", s.end))
		return
	}

	if len(logs) == 0 {
		s.log.Warn("Attempted to retrieve audit logs but received 0 logs", zap.Error(err), zap.String("hostname", hostname), zap.String("log", logName), zap.Time("startTime", s.start), zap.Time("endTime", s.end))
		return
	}

	plog, err := mongodbAuditEventToLogData(s.log,
		logs,
		pc,
		hostname,
		logName,
		clusterInfo)
	if err != nil {
		s.log.Warn("Failed to translate audit logs: "+logName, zap.Error(err))
		return
	}

	err = s.consumer.ConsumeLogs(context.Background(), plog)
	if err != nil {
		s.log.Error("Failed to consume logs", zap.Error(err))
	}
}
