// Licensed to The New Relic under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The New Relic licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// RacScraper contains the scraper for Oracle RAC-specific metrics
type RacScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig

	// For tracking rate-based metrics
	previousWaitEvents map[string]float64
	mutex              sync.RWMutex
}

// NewRacScraper creates a new RAC Scraper instance
func NewRacScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *RacScraper {
	return &RacScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
		previousWaitEvents:   make(map[string]float64),
	}
}

// ScrapeRacMetrics collects all Oracle RAC-specific metrics concurrently
func (s *RacScraper) ScrapeRacMetrics(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle RAC metrics scrape with concurrent processing")

	// Create context with timeout for concurrent operations
	scrapeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Channel to collect errors from all goroutines
	errorChan := make(chan []error, 4) // Buffer for 4 scraper functions
	var wg sync.WaitGroup

	// Launch goroutines for each metric group
	wg.Add(4)

	// Scrape ASM metrics concurrently
	go func() {
		defer wg.Done()
		if errs := s.scrapeASMDiskGroups(scrapeCtx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape cluster wait events concurrently
	go func() {
		defer wg.Done()
		if errs := s.scrapeClusterWaitEvents(scrapeCtx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape instance status concurrently
	go func() {
		defer wg.Done()
		if errs := s.scrapeInstanceStatus(scrapeCtx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape service failover status concurrently
	go func() {
		defer wg.Done()
		if errs := s.scrapeActiveServices(scrapeCtx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Collect all errors
	var allErrors []error
	for errors := range errorChan {
		if errors != nil {
			allErrors = append(allErrors, errors...)
		}
	}

	// Check if context was cancelled
	if scrapeCtx.Err() != nil {
		s.logger.Warn("RAC metrics scrape context cancelled", zap.Error(scrapeCtx.Err()))
		allErrors = append(allErrors, scrapeCtx.Err())
	}

	s.logger.Debug("Completed Oracle RAC metrics scrape",
		zap.Int("error_count", len(allErrors)),
		zap.Bool("concurrent_processing", true))
	return allErrors
}

// scrapeASMDiskGroups implements Feature 1: Automatic Storage Management (ASM) Monitoring
func (s *RacScraper) scrapeASMDiskGroups(ctx context.Context) []error {
	s.logger.Debug("Begin ASM disk group metrics scrape")

	var scrapeErrors []error

	rows, err := s.db.QueryContext(ctx, queries.ASMDiskGroupSQL)
	if err != nil {
		s.logger.Error("Failed to execute ASM disk group query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var name sql.NullString
		var totalMB sql.NullFloat64
		var freeMB sql.NullFloat64
		var offlineDisks sql.NullFloat64

		if err := rows.Scan(&name, &totalMB, &freeMB, &offlineDisks); err != nil {
			s.logger.Error("Failed to scan ASM disk group row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		if !name.Valid {
			s.logger.Debug("Skipping ASM disk group with null name")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())

		// Record total MB metric
		if totalMB.Valid {
			s.mb.RecordNewrelicoracledbAsmDiskgroupTotalMbDataPoint(now, totalMB.Float64, s.instanceName, name.String)
		}

		// Record free MB metric
		if freeMB.Valid {
			s.mb.RecordNewrelicoracledbAsmDiskgroupFreeMbDataPoint(now, freeMB.Float64, s.instanceName, name.String)
		}

		// Record offline disks metric
		if offlineDisks.Valid {
			s.mb.RecordNewrelicoracledbAsmDiskgroupOfflineDisksDataPoint(now, int64(offlineDisks.Float64), s.instanceName, name.String)
		}

		s.logger.Debug("Processed ASM disk group",
			zap.String("name", name.String),
			zap.Float64("total_mb", totalMB.Float64),
			zap.Float64("free_mb", freeMB.Float64),
			zap.Float64("offline_disks", offlineDisks.Float64))
	}

	return scrapeErrors
}

// scrapeClusterWaitEvents implements Feature 2: Detailed Cluster Wait Events
func (s *RacScraper) scrapeClusterWaitEvents(ctx context.Context) []error {
	s.logger.Debug("Begin cluster wait events metrics scrape")

	var scrapeErrors []error

	rows, err := s.db.QueryContext(ctx, queries.ClusterWaitEventsSQL)
	if err != nil {
		s.logger.Error("Failed to execute cluster wait events query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for rows.Next() {
		var instID sql.NullString
		var event sql.NullString
		var totalWaits sql.NullFloat64
		var timeWaitedMicro sql.NullFloat64

		if err := rows.Scan(&instID, &event, &totalWaits, &timeWaitedMicro); err != nil {
			s.logger.Error("Failed to scan cluster wait event row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		if !instID.Valid || !event.Valid || !timeWaitedMicro.Valid {
			s.logger.Debug("Skipping cluster wait event with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		instanceIDStr := instID.String
		eventName := event.String

		// Create unique key for this instance/event combination
		key := fmt.Sprintf("%s:%s", instanceIDStr, eventName)

		// Calculate rate for TIME_WAITED_MICRO (cumulative counter)
		if previous, exists := s.previousWaitEvents[key]; exists {
			if timeWaitedMicro.Float64 >= previous {
				rate := timeWaitedMicro.Float64 - previous

				// Record the rate-based metric
				s.mb.RecordNewrelicoracledbRacWaitTimeDataPoint(now, rate, s.instanceName, instanceIDStr, eventName)

				s.logger.Debug("Processed cluster wait event",
					zap.String("instance_id", instanceIDStr),
					zap.String("event", eventName),
					zap.Float64("time_waited_micro", timeWaitedMicro.Float64),
					zap.Float64("rate_micro_per_second", rate))
			}
		}

		// Store current value for next iteration
		s.previousWaitEvents[key] = timeWaitedMicro.Float64

		// Also record total waits if available
		if totalWaits.Valid {
			s.mb.RecordNewrelicoracledbRacTotalWaitsDataPoint(now, int64(totalWaits.Float64), s.instanceName, instanceIDStr, eventName)
		}
	}

	return scrapeErrors
}

// scrapeInstanceStatus implements Feature 3A: Node Availability (Instance Status)
func (s *RacScraper) scrapeInstanceStatus(ctx context.Context) []error {
	s.logger.Debug("Begin RAC instance status metrics scrape")

	var scrapeErrors []error

	rows, err := s.db.QueryContext(ctx, queries.RACInstanceStatusSQL)
	if err != nil {
		s.logger.Error("Failed to execute RAC instance status query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var instID sql.NullString
		var instanceName sql.NullString
		var hostName sql.NullString
		var status sql.NullString
		var startupTime sql.NullTime
		var databaseStatus sql.NullString
		var activeState sql.NullString
		var logins sql.NullString
		var archiver sql.NullString
		var version sql.NullString

		if err := rows.Scan(&instID, &instanceName, &hostName, &status, &startupTime, &databaseStatus, &activeState, &logins, &archiver, &version); err != nil {
			s.logger.Error("Failed to scan RAC instance status row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		if !instID.Valid || !status.Valid {
			s.logger.Debug("Skipping RAC instance with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		instanceIDStr := instID.String
		statusStr := status.String
		instanceNameStr := ""
		hostNameStr := ""
		databaseStatusStr := ""
		activeStateStr := ""
		loginsStr := ""
		archiverStr := ""

		if instanceName.Valid {
			instanceNameStr = instanceName.String
		}
		if hostName.Valid {
			hostNameStr = hostName.String
		}
		if databaseStatus.Valid {
			databaseStatusStr = databaseStatus.String
		}
		if activeState.Valid {
			activeStateStr = activeState.String
		}
		if logins.Valid {
			loginsStr = logins.String
		}
		if archiver.Valid {
			archiverStr = archiver.String
		}

		versionStr := ""
		if version.Valid {
			versionStr = version.String
		}

		// Convert status to numeric: OPEN = 1, anything else = 0
		var statusValue int64 = 0
		if strings.ToUpper(statusStr) == "OPEN" {
			statusValue = 1
		}

		// Record the original status metric
		s.mb.RecordNewrelicoracledbRacInstanceStatusDataPoint(now, statusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, statusStr)

		// Calculate and record uptime
		if startupTime.Valid {
			uptime := time.Since(startupTime.Time).Seconds()
			s.mb.RecordNewrelicoracledbRacInstanceUptimeSecondsDataPoint(now, int64(uptime), s.instanceName, instanceIDStr, instanceNameStr, hostNameStr)
		}

		// Record database status (1=ACTIVE, 0=other)
		var dbStatusValue int64 = 0
		if strings.ToUpper(databaseStatusStr) == "ACTIVE" {
			dbStatusValue = 1
		}
		s.mb.RecordNewrelicoracledbRacInstanceDatabaseStatusDataPoint(now, dbStatusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, databaseStatusStr)

		// Record active state (1=NORMAL, 0=other)
		var activeStateValue int64 = 0
		if strings.ToUpper(activeStateStr) == "NORMAL" {
			activeStateValue = 1
		}
		s.mb.RecordNewrelicoracledbRacInstanceActiveStateDataPoint(now, activeStateValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, activeStateStr)

		// Record logins status (1=ALLOWED, 0=RESTRICTED)
		var loginsValue int64 = 0
		if strings.ToUpper(loginsStr) == "ALLOWED" {
			loginsValue = 1
		}
		s.mb.RecordNewrelicoracledbRacInstanceLoginsAllowedDataPoint(now, loginsValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, loginsStr)

		// Record archiver status (1=STARTED, 0=STOPPED)
		var archiverValue int64 = 0
		if strings.ToUpper(archiverStr) == "STARTED" {
			archiverValue = 1
		}
		s.mb.RecordNewrelicoracledbRacInstanceArchiverStartedDataPoint(now, archiverValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, archiverStr)

		// Record version info
		s.mb.RecordNewrelicoracledbRacInstanceVersionInfoDataPoint(now, 1, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, versionStr)

		s.logger.Debug("Processed RAC instance status",
			zap.String("instance_id", instanceIDStr),
			zap.String("instance_name", instanceNameStr),
			zap.String("host_name", hostNameStr),
			zap.String("status", statusStr),
			zap.Int64("status_value", statusValue),
			zap.String("database_status", databaseStatusStr),
			zap.String("active_state", activeStateStr),
			zap.String("logins", loginsStr),
			zap.String("archiver", archiverStr))
	}

	return scrapeErrors
}

// scrapeActiveServices implements Feature 3B: Service Failover Status
func (s *RacScraper) scrapeActiveServices(ctx context.Context) []error {
	s.logger.Debug("Begin RAC active services metrics scrape")

	var scrapeErrors []error

	rows, err := s.db.QueryContext(ctx, queries.RACActiveServicesSQL)
	if err != nil {
		s.logger.Error("Failed to execute RAC active services query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var serviceName sql.NullString
		var instID sql.NullString
		var failoverMethod sql.NullString
		var failoverType sql.NullString
		var goal sql.NullString
		var networkName sql.NullString
		var creationDate sql.NullString
		var failoverRetries sql.NullString
		var failoverDelay sql.NullString
		var clbGoal sql.NullString

		if err := rows.Scan(&serviceName, &instID, &failoverMethod, &failoverType, &goal, &networkName, &creationDate, &failoverRetries, &failoverDelay, &clbGoal); err != nil {
			s.logger.Error("Failed to scan RAC active service row", zap.Error(err))
			scrapeErrors = append(scrapeErrors, err)
			continue
		}

		if !serviceName.Valid || !instID.Valid {
			s.logger.Debug("Skipping RAC active service with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		serviceNameStr := serviceName.String
		instanceIDStr := instID.String
		failoverMethodStr := ""
		failoverTypeStr := ""
		goalStr := ""
		networkNameStr := ""
		creationDateStr := ""
		failoverRetriesStr := ""
		failoverDelayStr := ""
		clbGoalStr := ""

		if failoverMethod.Valid {
			failoverMethodStr = failoverMethod.String
		}
		if failoverType.Valid {
			failoverTypeStr = failoverType.String
		}
		if goal.Valid {
			goalStr = goal.String
		}
		if networkName.Valid {
			networkNameStr = networkName.String
		}
		if creationDate.Valid {
			creationDateStr = creationDate.String
		}
		if failoverRetries.Valid {
			failoverRetriesStr = failoverRetries.String
		}
		if failoverDelay.Valid {
			failoverDelayStr = failoverDelay.String
		}
		if clbGoal.Valid {
			clbGoalStr = clbGoal.String
		}

		// Record the instance ID where this service is currently running
		instanceIDInt, _ := strconv.ParseFloat(instanceIDStr, 64)
		s.mb.RecordNewrelicoracledbRacServiceInstanceIDDataPoint(now, instanceIDInt, s.instanceName, serviceNameStr, instanceIDStr)

		// Record service failover configuration
		s.mb.RecordNewrelicoracledbRacServiceFailoverConfigDataPoint(now, int64(1), s.instanceName, serviceNameStr, instanceIDStr, failoverMethodStr, failoverTypeStr, goalStr)

		// Record service network name
		s.mb.RecordNewrelicoracledbRacServiceNetworkConfigDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr, networkNameStr)

		// Record service creation date
		s.mb.RecordNewrelicoracledbRacServiceCreationAgeDaysDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr)

		// Record failover retries
		var failoverRetriesValue int64 = 0
		if failoverRetriesStr != "" {
			if retriesInt, err := strconv.ParseInt(failoverRetriesStr, 10, 64); err == nil {
				failoverRetriesValue = retriesInt
			}
		}
		s.mb.RecordNewrelicoracledbRacServiceFailoverRetriesDataPoint(now, failoverRetriesValue, s.instanceName, serviceNameStr, instanceIDStr)

		// Record failover delay
		var failoverDelayValue int64 = 0
		if failoverDelayStr != "" {
			if delayInt, err := strconv.ParseInt(failoverDelayStr, 10, 64); err == nil {
				failoverDelayValue = delayInt
			}
		}
		s.mb.RecordNewrelicoracledbRacServiceFailoverDelaySecondsDataPoint(now, failoverDelayValue, s.instanceName, serviceNameStr, instanceIDStr)

		// Record CLB goal
		s.mb.RecordNewrelicoracledbRacServiceClbConfigDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr, clbGoalStr)

		s.logger.Debug("Processed RAC active service",
			zap.String("service_name", serviceNameStr),
			zap.String("instance_id", instanceIDStr),
			zap.String("failover_method", failoverMethodStr),
			zap.String("failover_type", failoverTypeStr),
			zap.String("goal", goalStr),
			zap.String("network_name", networkNameStr),
			zap.String("creation_date", creationDateStr),
			zap.String("failover_retries", failoverRetriesStr),
			zap.String("failover_delay", failoverDelayStr),
			zap.String("clb_goal", clbGoalStr))
	}

	return scrapeErrors
}
