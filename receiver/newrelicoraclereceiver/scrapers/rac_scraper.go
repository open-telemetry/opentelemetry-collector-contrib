// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

// NewRacScraper creates a new RAC metrics scraper
func NewRacScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *RacScraper {
	return &RacScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: config,
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

	if racEnabled {
		s.logger.Info("Oracle RAC mode detected", zap.String("cluster_database", clusterDB.String))
	}

	return racEnabled, nil
}

// isASMAvailable checks if ASM (Automatic Storage Management) is available
func (s *RacScraper) isASMAvailable(ctx context.Context) (bool, error) {
	rows, err := s.db.QueryContext(ctx, queries.ASMDetectionSQL)
	if err != nil {
		s.logger.Debug("Failed to execute ASM detection query (expected if ASM not configured)", zap.Error(err))
		return false, nil // Don't treat as error - ASM might not be configured
	}
	defer rows.Close()

	var asmCount int
	if rows.Next() {
		if err := rows.Scan(&asmCount); err != nil {
			s.logger.Error("Failed to scan ASM detection result", zap.Error(err))
			return false, err
		}
	}

	asmAvailable := asmCount > 0
	if asmAvailable {
		s.logger.Info("Oracle ASM storage detected", zap.Int("diskgroup_tables_found", asmCount))
	}

	return asmAvailable, nil
}

// ScrapeRacMetrics collects Oracle RAC and ASM metrics based on availability
// It detects both RAC cluster configuration and ASM storage management,
// running appropriate scrapers for the detected environment
func (s *RacScraper) ScrapeRacMetrics(ctx context.Context) []error {
	// Check if RAC is enabled
	racEnabled, err := s.isRacEnabled(ctx)
	if err != nil {
		s.logger.Error("Failed to detect RAC mode", zap.Error(err))
		return []error{err}
	}

	// Check if ASM is available
	asmAvailable, err := s.isASMAvailable(ctx)
	if err != nil {
		s.logger.Error("Failed to detect ASM availability", zap.Error(err))
		return []error{err}
	}

	// Skip if neither RAC nor ASM is available
	if !racEnabled && !asmAvailable {
		return nil
	}

	// Check if context is already cancelled
	if ctx.Err() != nil {
		return []error{ctx.Err()}
	}

	// Determine which scrapers to run based on capabilities
	var scrapers []func(context.Context) []error
	var scraperCount int

	// ASM scraper runs for both RAC and standalone ASM
	if asmAvailable {
		scrapers = append(scrapers, s.scrapeASMDiskGroups)
		scraperCount++
	}

	// RAC-specific scrapers only run in RAC mode
	if racEnabled {
		scrapers = append(scrapers,
			s.scrapeClusterWaitEvents,
			s.scrapeInstanceStatus,
			s.scrapeActiveServices,
		)
		scraperCount += 3
	}

	if scraperCount == 0 {
		return nil
	}

	// Launch concurrent scrapers
	errorChan := make(chan []error, scraperCount)
	var wg sync.WaitGroup
	wg.Add(scraperCount)

	for _, scraper := range scrapers {
		go func(fn func(context.Context) []error) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errorChan <- []error{ctx.Err()}
				return
			default:
			}
			if errs := fn(ctx); len(errs) > 0 {
				errorChan <- errs
			} else {
				errorChan <- nil
			}
		}(scraper)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Collect errors
	var allErrors []error
	for errors := range errorChan {
		if errors != nil {
			allErrors = append(allErrors, errors...)
		}
	}

	if ctx.Err() != nil {
		allErrors = append(allErrors, ctx.Err())
	}

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
	// Check if any ASM metrics are enabled before querying
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupTotalMb.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupFreeMb.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupOfflineDisks.Enabled {
		return nil
	}

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
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())

		// Record metrics only if enabled
		if totalMB.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupTotalMb.Enabled {
			s.mb.RecordNewrelicoracledbAsmDiskgroupTotalMbDataPoint(now, totalMB.Float64, s.instanceName, name.String)
		}
		if freeMB.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupFreeMb.Enabled {
			s.mb.RecordNewrelicoracledbAsmDiskgroupFreeMbDataPoint(now, freeMB.Float64, s.instanceName, name.String)
		}
		if offlineDisks.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbAsmDiskgroupOfflineDisks.Enabled {
			s.mb.RecordNewrelicoracledbAsmDiskgroupOfflineDisksDataPoint(now, int64(offlineDisks.Float64), s.instanceName, name.String)
		}
	}

	return scrapeErrors
}

// scrapeClusterWaitEvents implements Feature 2: Detailed Cluster Wait Events
func (s *RacScraper) scrapeClusterWaitEvents(ctx context.Context) []error {
	// Check if cluster wait event metrics are enabled
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbRacWaitTime.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacTotalWaits.Enabled {
		return nil
	}

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
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		instanceIDStr := instID.String
		eventName := event.String

		// Record metrics only if enabled
		if timeWaitedMicro.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbRacWaitTime.Enabled {
			s.mb.RecordNewrelicoracledbRacWaitTimeDataPoint(now, timeWaitedMicro.Float64, s.instanceName, instanceIDStr, eventName)
		}
		if totalWaits.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbRacTotalWaits.Enabled {
			s.mb.RecordNewrelicoracledbRacTotalWaitsDataPoint(now, int64(totalWaits.Float64), s.instanceName, instanceIDStr, eventName)
		}
	}

	return scrapeErrors
}

// scrapeInstanceStatus implements Feature 3A: Node Availability (Instance Status)
func (s *RacScraper) scrapeInstanceStatus(ctx context.Context) []error {
	// Check if any instance status metrics are enabled
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceStatus.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceUptimeSeconds.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceDatabaseStatus.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceActiveState.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceLoginsAllowed.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceArchiverStarted.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceVersionInfo.Enabled {
		return nil
	}

	var scrapeErrors []error

	rows, err := s.executeQuery(ctx, queries.RACInstanceStatusSQL, "RAC instance status")
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var instID, instanceName, hostName, status sql.NullString
		var startupTime sql.NullTime
		var databaseStatus, activeState, logins, archiver, version sql.NullString

		if err := rows.Scan(&instID, &instanceName, &hostName, &status, &startupTime, &databaseStatus, &activeState, &logins, &archiver, &version); err != nil {
			scrapeErrors = s.handleScanError(err, scrapeErrors, "RAC instance status")
			continue
		}

		if !instID.Valid || !status.Valid {
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

		// Record metrics only if enabled
		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceStatus.Enabled {
			statusValue := stringStatusToBinary(statusStr, "OPEN")
			s.mb.RecordNewrelicoracledbRacInstanceStatusDataPoint(now, statusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, statusStr)
		}

		if startupTime.Valid && s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceUptimeSeconds.Enabled {
			uptime := time.Since(startupTime.Time).Seconds()
			s.mb.RecordNewrelicoracledbRacInstanceUptimeSecondsDataPoint(now, int64(uptime), s.instanceName, instanceIDStr, instanceNameStr, hostNameStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceDatabaseStatus.Enabled {
			dbStatusValue := stringStatusToBinary(databaseStatusStr, "ACTIVE")
			s.mb.RecordNewrelicoracledbRacInstanceDatabaseStatusDataPoint(now, dbStatusValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, databaseStatusStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceActiveState.Enabled {
			activeStateValue := stringStatusToBinary(activeStateStr, "NORMAL")
			s.mb.RecordNewrelicoracledbRacInstanceActiveStateDataPoint(now, activeStateValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, activeStateStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceLoginsAllowed.Enabled {
			loginsValue := stringStatusToBinary(loginsStr, "ALLOWED")
			s.mb.RecordNewrelicoracledbRacInstanceLoginsAllowedDataPoint(now, loginsValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, loginsStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceArchiverStarted.Enabled {
			archiverValue := stringStatusToBinary(archiverStr, "STARTED")
			s.mb.RecordNewrelicoracledbRacInstanceArchiverStartedDataPoint(now, archiverValue, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, archiverStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacInstanceVersionInfo.Enabled {
			s.mb.RecordNewrelicoracledbRacInstanceVersionInfoDataPoint(now, 1, s.instanceName, instanceIDStr, instanceNameStr, hostNameStr, versionStr)
		}
	}

	return scrapeErrors
}

// scrapeActiveServices implements Feature 3B: Service Failover Status
func (s *RacScraper) scrapeActiveServices(ctx context.Context) []error {
	// Check if any service metrics are enabled
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceInstanceID.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverConfig.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceNetworkConfig.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceCreationAgeDays.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverRetries.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverDelaySeconds.Enabled &&
		!s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceClbConfig.Enabled {
		return nil
	}

	var scrapeErrors []error

	rows, err := s.executeQuery(ctx, queries.RACActiveServicesSQL, "RAC active services")
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var serviceName, instID, failoverMethod, failoverType, goal sql.NullString
		var networkName, creationDate, failoverRetries, failoverDelay, clbGoal sql.NullString

		if err := rows.Scan(&serviceName, &instID, &failoverMethod, &failoverType, &goal, &networkName, &creationDate, &failoverRetries, &failoverDelay, &clbGoal); err != nil {
			scrapeErrors = s.handleScanError(err, scrapeErrors, "RAC active service")
			continue
		}

		if !serviceName.Valid || !instID.Valid {
			continue
		}

		now := pcommon.NewTimestampFromTime(time.Now())
		serviceNameStr := serviceName.String
		instanceIDStr := instID.String

		// Record metrics only if enabled
		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceInstanceID.Enabled {
			instanceIDInt, _ := strconv.ParseFloat(instanceIDStr, 64)
			s.mb.RecordNewrelicoracledbRacServiceInstanceIDDataPoint(now, instanceIDInt, s.instanceName, serviceNameStr, instanceIDStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverConfig.Enabled {
			s.mb.RecordNewrelicoracledbRacServiceFailoverConfigDataPoint(now, int64(1), s.instanceName, serviceNameStr, instanceIDStr,
				nullStringToString(failoverMethod), nullStringToString(failoverType), nullStringToString(goal))
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceNetworkConfig.Enabled {
			s.mb.RecordNewrelicoracledbRacServiceNetworkConfigDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr, nullStringToString(networkName))
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceCreationAgeDays.Enabled {
			s.mb.RecordNewrelicoracledbRacServiceCreationAgeDaysDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr)
		}

		// Parse and record failover metrics only if enabled
		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverRetries.Enabled {
			var failoverRetriesValue int64 = 0
			if failoverRetriesStr := nullStringToString(failoverRetries); failoverRetriesStr != "" {
				if retriesInt, err := strconv.ParseInt(failoverRetriesStr, 10, 64); err == nil {
					failoverRetriesValue = retriesInt
				}
			}
			s.mb.RecordNewrelicoracledbRacServiceFailoverRetriesDataPoint(now, failoverRetriesValue, s.instanceName, serviceNameStr, instanceIDStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceFailoverDelaySeconds.Enabled {
			var failoverDelayValue int64 = 0
			if failoverDelayStr := nullStringToString(failoverDelay); failoverDelayStr != "" {
				if delayInt, err := strconv.ParseInt(failoverDelayStr, 10, 64); err == nil {
					failoverDelayValue = delayInt
				}
			}
			s.mb.RecordNewrelicoracledbRacServiceFailoverDelaySecondsDataPoint(now, failoverDelayValue, s.instanceName, serviceNameStr, instanceIDStr)
		}

		if s.metricsBuilderConfig.Metrics.NewrelicoracledbRacServiceClbConfig.Enabled {
			s.mb.RecordNewrelicoracledbRacServiceClbConfigDataPoint(now, 1, s.instanceName, serviceNameStr, instanceIDStr, nullStringToString(clbGoal))
		}
	}

	return scrapeErrors
}
