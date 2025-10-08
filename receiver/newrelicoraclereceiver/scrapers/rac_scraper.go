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

	// Cache for RAC mode detection to avoid repeated queries
	isRacMode    *bool
	racModeMutex sync.RWMutex
}

// NewRacScraper creates a new RAC Scraper instance
func NewRacScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *RacScraper {
	return &RacScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// isRacEnabled checks if Oracle is running in RAC mode by querying the cluster_database parameter
func (s *RacScraper) isRacEnabled(ctx context.Context) (bool, error) {
	s.racModeMutex.RLock()
	if s.isRacMode != nil {
		result := *s.isRacMode
		s.racModeMutex.RUnlock()
		return result, nil
	}
	s.racModeMutex.RUnlock()

	s.racModeMutex.Lock()
	defer s.racModeMutex.Unlock()

	// Double-check pattern to avoid race conditions
	if s.isRacMode != nil {
		return *s.isRacMode, nil
	}

	s.logger.Debug("Checking if Oracle is running in RAC mode")

	rows, err := s.db.QueryContext(ctx, queries.RACDetectionSQL)
	if err != nil {
		s.logger.Error("Failed to execute RAC detection query", zap.Error(err))
		return false, err
	}
	defer rows.Close()

	var clusterDB sql.NullString
	if rows.Next() {
		if err := rows.Scan(&clusterDB); err != nil {
			s.logger.Error("Failed to scan RAC detection result", zap.Error(err))
			return false, err
		}
	}

	// RAC is enabled if cluster_database parameter is TRUE
	racEnabled := clusterDB.Valid && strings.ToUpper(clusterDB.String) == "TRUE"
	s.isRacMode = &racEnabled

	s.logger.Info("RAC mode detection completed",
		zap.Bool("rac_enabled", racEnabled),
		zap.String("cluster_database_value", clusterDB.String))

	return racEnabled, nil
}

// ScrapeRacMetrics collects all Oracle RAC-specific metrics concurrently
func (s *RacScraper) ScrapeRacMetrics(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle RAC metrics scrape")

	// First check if RAC is enabled
	racEnabled, err := s.isRacEnabled(ctx)
	if err != nil {
		s.logger.Error("Failed to detect RAC mode", zap.Error(err))
		return []error{err}
	}

	if !racEnabled {
		s.logger.Debug("Oracle is not running in RAC mode, skipping RAC-specific metrics")
		return nil
	}

	s.logger.Debug("Oracle RAC mode detected, proceeding with RAC metrics collection")

	// Check if context is already cancelled before starting goroutines
	if ctx.Err() != nil {
		s.logger.Warn("Context already cancelled before starting RAC metrics collection", zap.Error(ctx.Err()))
		return []error{ctx.Err()}
	}

	// Channel to collect errors from all goroutines
	errorChan := make(chan []error, 4) // Buffer for 4 scraper functions
	var wg sync.WaitGroup

	// Launch goroutines for each metric group
	wg.Add(4)

	// Scrape ASM metrics concurrently
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			errorChan <- []error{ctx.Err()}
			return
		default:
		}
		if errs := s.scrapeASMDiskGroups(ctx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape cluster wait events concurrently
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			errorChan <- []error{ctx.Err()}
			return
		default:
		}
		if errs := s.scrapeClusterWaitEvents(ctx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape instance status concurrently
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			errorChan <- []error{ctx.Err()}
			return
		default:
		}
		if errs := s.scrapeInstanceStatus(ctx); len(errs) > 0 {
			errorChan <- errs
		} else {
			errorChan <- nil
		}
	}()

	// Scrape service failover status concurrently
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			errorChan <- []error{ctx.Err()}
			return
		default:
		}
		if errs := s.scrapeActiveServices(ctx); len(errs) > 0 {
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
	if ctx.Err() != nil {
		s.logger.Warn("RAC metrics scrape context cancelled", zap.Error(ctx.Err()))
		allErrors = append(allErrors, ctx.Err())
	}

	s.logger.Debug("Completed Oracle RAC metrics scrape",
		zap.Int("error_count", len(allErrors)),
		zap.Bool("concurrent_processing", true))
	return allErrors
}

// Utility functions to reduce code duplication

// executeQuery executes a database query with proper error handling and logging
func (s *RacScraper) executeQuery(ctx context.Context, query string, queryType string) (*sql.Rows, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		s.logger.Error("Failed to execute "+queryType+" query", zap.Error(err))
		return nil, err
	}
	return rows, nil
}

// handleScanError handles row scanning errors with consistent logging and error collection
func (s *RacScraper) handleScanError(err error, scrapeErrors []error, context string) []error {
	s.logger.Error("Failed to scan "+context+" row", zap.Error(err))
	return append(scrapeErrors, err)
}

// nullStringToString safely converts sql.NullString to string
func nullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// stringStatusToBinary converts string status to binary value (1 for match, 0 for no match)
func stringStatusToBinary(status, expectedValue string) int64 {
	if strings.ToUpper(status) == strings.ToUpper(expectedValue) {
		return 1
	}
	return 0
}

// scrapeASMDiskGroups implements Feature 1: Automatic Storage Management (ASM) Monitoring
func (s *RacScraper) scrapeASMDiskGroups(ctx context.Context) []error {
	s.logger.Debug("Begin ASM disk group metrics scrape")

	var scrapeErrors []error

	rows, err := s.executeQuery(ctx, queries.ASMDiskGroupSQL, "ASM disk group")
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var name sql.NullString
		var totalMB sql.NullFloat64
		var freeMB sql.NullFloat64
		var offlineDisks sql.NullFloat64

		if err := rows.Scan(&name, &totalMB, &freeMB, &offlineDisks); err != nil {
			scrapeErrors = s.handleScanError(err, scrapeErrors, "ASM disk group")
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

	rows, err := s.executeQuery(ctx, queries.ClusterWaitEventsSQL, "cluster wait events")
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var instID sql.NullString
		var event sql.NullString
		var totalWaits sql.NullFloat64
		var timeWaitedMicro sql.NullFloat64

		if err := rows.Scan(&instID, &event, &totalWaits, &timeWaitedMicro); err != nil {
			scrapeErrors = s.handleScanError(err, scrapeErrors, "cluster wait event")
			continue
		}

		if !instID.Valid || !event.Valid {
			s.logger.Debug("Skipping cluster wait event with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		instanceIDStr := instID.String
		eventName := event.String

		// Record the raw time waited in microseconds (cumulative counter)
		if timeWaitedMicro.Valid {
			s.mb.RecordNewrelicoracledbRacWaitTimeDataPoint(now, timeWaitedMicro.Float64, s.instanceName, instanceIDStr, eventName)
		}

		// Record total waits if available
		if totalWaits.Valid {
			s.mb.RecordNewrelicoracledbRacTotalWaitsDataPoint(now, int64(totalWaits.Float64), s.instanceName, instanceIDStr, eventName)
		}

		s.logger.Debug("Processed cluster wait event",
			zap.String("instance_id", instanceIDStr),
			zap.String("event", eventName),
			zap.Float64("time_waited_micro", timeWaitedMicro.Float64),
			zap.Float64("total_waits", totalWaits.Float64))
	}

	return scrapeErrors
}

// scrapeInstanceStatus implements Feature 3A: Node Availability (Instance Status)
func (s *RacScraper) scrapeInstanceStatus(ctx context.Context) []error {
	s.logger.Debug("Begin RAC instance status metrics scrape")

	var scrapeErrors []error

	rows, err := s.executeQuery(ctx, queries.RACInstanceStatusSQL, "RAC instance status")
	if err != nil {
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
			scrapeErrors = s.handleScanError(err, scrapeErrors, "RAC instance status")
			continue
		}

		if !instID.Valid || !status.Valid {
			s.logger.Debug("Skipping RAC instance with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		instanceIDStr := instID.String
		statusStr := status.String
		instanceNameStr := nullStringToString(instanceName)
		hostNameStr := nullStringToString(hostName)
		databaseStatusStr := nullStringToString(databaseStatus)
		activeStateStr := nullStringToString(activeState)
		loginsStr := nullStringToString(logins)
		archiverStr := nullStringToString(archiver)
		versionStr := nullStringToString(version)

		// Convert status to numeric: OPEN = 1, anything else = 0
		statusValue := stringStatusToBinary(statusStr, "OPEN")

		// Record the original status metric
		s.mb.RecordNewrelicoracledbRacInstanceStatusDataPoint(now, statusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, statusStr)

		// Calculate and record uptime
		if startupTime.Valid {
			uptime := time.Since(startupTime.Time).Seconds()
			s.mb.RecordNewrelicoracledbRacInstanceUptimeSecondsDataPoint(now, int64(uptime), s.instanceName, instanceIDStr, instanceNameStr, hostNameStr)
		}

		// Record database status (1=ACTIVE, 0=other)
		dbStatusValue := stringStatusToBinary(databaseStatusStr, "ACTIVE")
		s.mb.RecordNewrelicoracledbRacInstanceDatabaseStatusDataPoint(now, dbStatusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, databaseStatusStr)

		// Record active state (1=NORMAL, 0=other)
		activeStateValue := stringStatusToBinary(activeStateStr, "NORMAL")
		s.mb.RecordNewrelicoracledbRacInstanceActiveStateDataPoint(now, activeStateValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, activeStateStr)

		// Record logins status (1=ALLOWED, 0=RESTRICTED)
		loginsValue := stringStatusToBinary(loginsStr, "ALLOWED")
		s.mb.RecordNewrelicoracledbRacInstanceLoginsAllowedDataPoint(now, loginsValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, loginsStr)

		// Record archiver status (1=STARTED, 0=STOPPED)
		archiverValue := stringStatusToBinary(archiverStr, "STARTED")
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

	rows, err := s.executeQuery(ctx, queries.RACActiveServicesSQL, "RAC active services")
	if err != nil {
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
			scrapeErrors = s.handleScanError(err, scrapeErrors, "RAC active service")
			continue
		}

		if !serviceName.Valid || !instID.Valid {
			s.logger.Debug("Skipping RAC active service with null values")
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		serviceNameStr := serviceName.String
		instanceIDStr := instID.String
		failoverMethodStr := nullStringToString(failoverMethod)
		failoverTypeStr := nullStringToString(failoverType)
		goalStr := nullStringToString(goal)
		networkNameStr := nullStringToString(networkName)
		creationDateStr := nullStringToString(creationDate)
		failoverRetriesStr := nullStringToString(failoverRetries)
		failoverDelayStr := nullStringToString(failoverDelay)
		clbGoalStr := nullStringToString(clbGoal)

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
