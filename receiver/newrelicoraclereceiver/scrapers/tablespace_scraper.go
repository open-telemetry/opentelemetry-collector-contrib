// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// TablespaceScraper handles Oracle tablespace metrics with environment detection
type TablespaceScraper struct {
	db           *sql.DB
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig

	// Environment capability caching
	isCDBCapable       *bool        // Cache for CDB capability check
	isPDBCapable       *bool        // Cache for PDB capability check
	environmentChecked bool         // Track if environment has been checked
	currentContainer   string       // Current container context (CDB$ROOT, FREEPDB1, etc.)
	currentContainerID string       // Current container ID (1, 3, etc.)
	contextChecked     bool         // Track if context has been checked
	detectionMutex     sync.RWMutex // Protect concurrent access to detection state
}

// NewTablespaceScraper creates a new tablespace scraper
func NewTablespaceScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *TablespaceScraper {
	return &TablespaceScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeTablespaceMetrics collects Oracle tablespace metrics with environment-aware execution
func (s *TablespaceScraper) ScrapeTablespaceMetrics(ctx context.Context) []error {
	var errors []error

	s.logger.Debug("Scraping Oracle tablespace metrics")

	// Check environment capability first
	if err := s.checkEnvironmentCapability(ctx); err != nil {
		s.logger.Error("Failed to check Oracle environment capabilities", zap.Error(err))
		return []error{err}
	}

	// Check current container context
	if err := s.checkCurrentContext(ctx); err != nil {
		s.logger.Error("Failed to check Oracle container context", zap.Error(err))
		return []error{err}
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	// Universal tablespace metrics (available in all Oracle versions)
	errors = append(errors, s.scrapeTablespaceUsageMetrics(ctx, now)...)
	errors = append(errors, s.scrapeGlobalNameTablespaceMetrics(ctx, now)...)
	errors = append(errors, s.scrapeDBIDTablespaceMetrics(ctx, now)...)

	// CDB-specific metrics (requires CDB capability)
	if s.isCDBSupported() {
		errors = append(errors, s.scrapeCDBDatafilesOfflineTablespaceMetrics(ctx, now)...)

		// PDB-specific metrics (requires PDB capability and appropriate context)
		if s.isPDBSupported() {
			errors = append(errors, s.scrapePDBDatafilesOfflineTablespaceMetrics(ctx, now)...)
			errors = append(errors, s.scrapePDBNonWriteTablespaceMetrics(ctx, now)...)
		} else {
			s.logger.Debug("PDB features not supported, skipping PDB tablespace metrics")
		}
	} else {
		s.logger.Debug("CDB features not supported, skipping CDB/PDB tablespace metrics")
	}

	return errors
}

// scrapeTablespaceUsageMetrics handles the main tablespace usage metrics
func (s *TablespaceScraper) scrapeTablespaceUsageMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if any tablespace usage metrics are enabled
	if !s.isAnyTablespaceMetricEnabled() {
		return nil
	}

	// Execute tablespace metrics query
	s.logger.Debug("Executing tablespace metrics query", zap.String("sql", queries.TablespaceMetricsSQL))

	rows, err := s.db.QueryContext(ctx, queries.TablespaceMetricsSQL)
	if err != nil {
		return []error{fmt.Errorf("error executing tablespace metrics query: %w", err)}
	}
	defer rows.Close()

	return s.processTablespaceUsageRows(rows, now)
}

// isAnyTablespaceMetricEnabled checks if any tablespace usage metric is enabled
func (s *TablespaceScraper) isAnyTablespaceMetricEnabled() bool {
	return s.config.Metrics.NewrelicoracledbTablespaceSpaceConsumedBytes.Enabled ||
		s.config.Metrics.NewrelicoracledbTablespaceSpaceReservedBytes.Enabled ||
		s.config.Metrics.NewrelicoracledbTablespaceSpaceUsedPercentage.Enabled ||
		s.config.Metrics.NewrelicoracledbTablespaceIsOffline.Enabled
}

// processTablespaceUsageRows processes the result rows for tablespace usage metrics
func (s *TablespaceScraper) processTablespaceUsageRows(rows *sql.Rows, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	for rows.Next() {
		var tablespaceName string
		var usedPercent, used, size, offline float64

		err := rows.Scan(&tablespaceName, &usedPercent, &used, &size, &offline)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning tablespace row: %w", err))
			continue
		}

		// Record tablespace metrics using conditional enablement
		s.logger.Debug("Tablespace metrics collected",
			zap.String("tablespace", tablespaceName),
			zap.Float64("used_percent", usedPercent),
			zap.Float64("used_bytes", used),
			zap.Float64("size_bytes", size),
			zap.Float64("offline", offline),
			zap.String("instance", s.instanceName))

		// Record enabled metrics only
		if s.config.Metrics.NewrelicoracledbTablespaceSpaceConsumedBytes.Enabled {
			s.mb.RecordNewrelicoracledbTablespaceSpaceConsumedBytesDataPoint(now, int64(used), s.instanceName, tablespaceName)
		}
		if s.config.Metrics.NewrelicoracledbTablespaceSpaceReservedBytes.Enabled {
			s.mb.RecordNewrelicoracledbTablespaceSpaceReservedBytesDataPoint(now, int64(size), s.instanceName, tablespaceName)
		}
		if s.config.Metrics.NewrelicoracledbTablespaceSpaceUsedPercentage.Enabled {
			s.mb.RecordNewrelicoracledbTablespaceSpaceUsedPercentageDataPoint(now, int64(usedPercent), s.instanceName, tablespaceName)
		}
		if s.config.Metrics.NewrelicoracledbTablespaceIsOffline.Enabled {
			s.mb.RecordNewrelicoracledbTablespaceIsOfflineDataPoint(now, int64(offline), s.instanceName, tablespaceName)
		}

		metricCount++
	}

	if err := rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating tablespace rows: %w", err))
	}

	s.logger.Debug("Collected Oracle tablespace metrics",
		zap.Int("metric_count", metricCount),
		zap.String("instance", s.instanceName))

	return errors
}

// scrapeGlobalNameTablespaceMetrics handles the global name tablespace metrics
func (s *TablespaceScraper) scrapeGlobalNameTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if metric is enabled
	if !s.config.Metrics.NewrelicoracledbTablespaceGlobalName.Enabled {
		return nil
	}

	return s.executeSimpleTablespaceQuery(ctx, now, queries.GlobalNameTablespaceSQL,
		"global name tablespace", s.processGlobalNameRow)
}

// scrapeDBIDTablespaceMetrics handles the DB ID tablespace metrics
func (s *TablespaceScraper) scrapeDBIDTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if metric is enabled
	if !s.config.Metrics.NewrelicoracledbTablespaceDbID.Enabled {
		return nil
	}

	return s.executeSimpleTablespaceQuery(ctx, now, queries.DBIDTablespaceSQL,
		"DB ID tablespace", s.processDBIDRow)
}

// scrapeCDBDatafilesOfflineTablespaceMetrics handles the CDB datafiles offline tablespace metrics
func (s *TablespaceScraper) scrapeCDBDatafilesOfflineTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if metric is enabled
	if !s.config.Metrics.NewrelicoracledbTablespaceOfflineCdbDatafiles.Enabled {
		return nil
	}

	return s.executeSimpleTablespaceQuery(ctx, now, queries.CDBDatafilesOfflineTablespaceSQL,
		"CDB datafiles offline tablespace", s.processCDBDatafilesOfflineRow)
}

// executeSimpleTablespaceQuery executes a query and processes results with the provided row processor
func (s *TablespaceScraper) executeSimpleTablespaceQuery(ctx context.Context, now pcommon.Timestamp,
	querySQL, description string, rowProcessor func(*sql.Rows, pcommon.Timestamp) error) []error {

	s.logger.Debug("Executing tablespace query",
		zap.String("sql", querySQL),
		zap.String("description", description))

	rows, err := s.db.QueryContext(ctx, querySQL)
	if err != nil {
		return []error{fmt.Errorf("error executing %s query: %w", description, err)}
	}
	defer rows.Close()

	var errors []error
	metricCount := 0

	for rows.Next() {
		if err := rowProcessor(rows, now); err != nil {
			errors = append(errors, fmt.Errorf("error processing %s row: %w", description, err))
			continue
		}
		metricCount++
	}

	if err := rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating %s rows: %w", description, err))
	}

	s.logger.Debug("Collected Oracle tablespace metrics",
		zap.Int("metric_count", metricCount),
		zap.String("instance", s.instanceName),
		zap.String("description", description))

	return errors
}

// Row processing functions for different metric types
func (s *TablespaceScraper) processGlobalNameRow(rows *sql.Rows, now pcommon.Timestamp) error {
	var tablespaceName, globalName string
	if err := rows.Scan(&tablespaceName, &globalName); err != nil {
		return err
	}

	s.logger.Debug("Global name tablespace metrics collected",
		zap.String("tablespace", tablespaceName),
		zap.String("global_name", globalName),
		zap.String("instance", s.instanceName))

	s.mb.RecordNewrelicoracledbTablespaceGlobalNameDataPoint(now, 1, s.instanceName, tablespaceName)
	return nil
}

func (s *TablespaceScraper) processDBIDRow(rows *sql.Rows, now pcommon.Timestamp) error {
	var tablespaceName string
	var dbID int64
	if err := rows.Scan(&tablespaceName, &dbID); err != nil {
		return err
	}

	s.logger.Debug("DB ID tablespace metrics collected",
		zap.String("tablespace", tablespaceName),
		zap.Int64("db_id", dbID),
		zap.String("instance", s.instanceName))

	s.mb.RecordNewrelicoracledbTablespaceDbIDDataPoint(now, dbID, s.instanceName, tablespaceName)
	return nil
}

func (s *TablespaceScraper) processCDBDatafilesOfflineRow(rows *sql.Rows, now pcommon.Timestamp) error {
	var offlineCount int64
	var tablespaceName string
	if err := rows.Scan(&offlineCount, &tablespaceName); err != nil {
		return err
	}

	s.logger.Debug("CDB datafiles offline tablespace metrics collected",
		zap.String("tablespace", tablespaceName),
		zap.Int64("offline_count", offlineCount),
		zap.String("instance", s.instanceName))

	s.mb.RecordNewrelicoracledbTablespaceOfflineCdbDatafilesDataPoint(now, offlineCount, s.instanceName, tablespaceName)
	return nil
}

// scrapePDBDatafilesOfflineTablespaceMetrics handles the PDB datafiles offline tablespace metrics
// Uses context-appropriate queries based on connection type (CDB$ROOT vs PDB)
func (s *TablespaceScraper) scrapePDBDatafilesOfflineTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if PDB datafiles offline metric is enabled
	if !s.config.Metrics.NewrelicoracledbTablespaceOfflinePdbDatafiles.Enabled {
		return nil
	}

	var querySQL string
	var queryDescription string

	// Select appropriate query based on connection context
	if s.isConnectedToCDBRoot() {
		querySQL = queries.PDBDatafilesOfflineTablespaceSQL
		queryDescription = "PDB datafiles offline tablespace (from CDB$ROOT)"
	} else if s.isConnectedToPDB() {
		querySQL = queries.PDBDatafilesOfflineCurrentContainerSQL
		queryDescription = "PDB datafiles offline tablespace (current container)"
	} else {
		s.logger.Debug("Not connected to appropriate context for PDB datafiles metrics",
			zap.String("current_container", s.currentContainer))
		return nil
	}

	// Execute PDB datafiles offline tablespace query
	s.logger.Debug("Executing PDB datafiles offline tablespace query",
		zap.String("sql", querySQL),
		zap.String("context", queryDescription))

	rows, err := s.db.QueryContext(ctx, querySQL)
	if err != nil {
		return []error{fmt.Errorf("error executing %s query: %w", queryDescription, err)}
	}
	defer rows.Close()

	return s.processPDBDatafilesOfflineRows(rows, now, queryDescription)
}

// processPDBDatafilesOfflineRows processes the result rows for PDB datafiles offline metrics
func (s *TablespaceScraper) processPDBDatafilesOfflineRows(rows *sql.Rows, now pcommon.Timestamp, context string) []error {
	var errors []error
	metricCount := 0

	for rows.Next() {
		var offlineCount int64
		var tablespaceName string

		err := rows.Scan(&offlineCount, &tablespaceName)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning %s row: %w", context, err))
			continue
		}

		// Record PDB datafiles offline metrics
		s.logger.Debug("PDB datafiles offline tablespace metrics collected",
			zap.String("tablespace", tablespaceName),
			zap.Int64("offline_count", offlineCount),
			zap.String("instance", s.instanceName),
			zap.String("context", context))

		// Record the PDB datafiles offline metric
		s.mb.RecordNewrelicoracledbTablespaceOfflinePdbDatafilesDataPoint(now, offlineCount, s.instanceName, tablespaceName)
		metricCount++
	}

	if err := rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating %s rows: %w", context, err))
	}

	s.logger.Debug("Collected Oracle PDB datafiles offline tablespace metrics",
		zap.Int("metric_count", metricCount),
		zap.String("instance", s.instanceName),
		zap.String("context", context))

	return errors
}

// scrapePDBNonWriteTablespaceMetrics handles the PDB non-write mode tablespace metrics
// Uses context-appropriate queries based on connection type (CDB$ROOT vs PDB)
func (s *TablespaceScraper) scrapePDBNonWriteTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if PDB non-write mode metric is enabled
	if !s.config.Metrics.NewrelicoracledbTablespacePdbNonWriteMode.Enabled {
		return nil
	}

	var querySQL string
	var queryDescription string

	// Select appropriate query based on connection context
	if s.isConnectedToCDBRoot() {
		querySQL = queries.PDBNonWriteTablespaceSQL
		queryDescription = "PDB non-write mode tablespace (from CDB$ROOT)"
	} else if s.isConnectedToPDB() {
		querySQL = queries.PDBNonWriteCurrentContainerSQL
		queryDescription = "PDB non-write mode tablespace (current container)"
	} else {
		s.logger.Debug("Not connected to appropriate context for PDB non-write mode metrics",
			zap.String("current_container", s.currentContainer))
		return nil
	}

	// Execute PDB non-write mode tablespace query
	s.logger.Debug("Executing PDB non-write mode tablespace query",
		zap.String("sql", querySQL),
		zap.String("context", queryDescription))

	rows, err := s.db.QueryContext(ctx, querySQL)
	if err != nil {
		return []error{fmt.Errorf("error executing %s query: %w", queryDescription, err)}
	}
	defer rows.Close()

	return s.processPDBNonWriteRows(rows, now, queryDescription)
}

// processPDBNonWriteRows processes the result rows for PDB non-write mode metrics
func (s *TablespaceScraper) processPDBNonWriteRows(rows *sql.Rows, now pcommon.Timestamp, context string) []error {
	var errors []error
	metricCount := 0

	for rows.Next() {
		var tablespaceName string
		var nonWriteCount int64

		err := rows.Scan(&tablespaceName, &nonWriteCount)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning %s row: %w", context, err))
			continue
		}

		// Record PDB non-write mode metrics
		s.logger.Debug("PDB non-write mode tablespace metrics collected",
			zap.String("tablespace", tablespaceName),
			zap.Int64("non_write_count", nonWriteCount),
			zap.String("instance", s.instanceName),
			zap.String("context", context))

		// Record the PDB non-write mode metric
		s.mb.RecordNewrelicoracledbTablespacePdbNonWriteModeDataPoint(now, nonWriteCount, s.instanceName, tablespaceName)
		metricCount++
	}

	if err := rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating %s rows: %w", context, err))
	}

	s.logger.Debug("Collected Oracle PDB non-write mode tablespace metrics",
		zap.Int("metric_count", metricCount),
		zap.String("instance", s.instanceName),
		zap.String("context", context))

	return errors
}

// Environment detection methods following container_scraper pattern

// checkEnvironmentCapability checks if the Oracle database supports CDB/PDB features
func (s *TablespaceScraper) checkEnvironmentCapability(ctx context.Context) error {
	s.detectionMutex.Lock()
	defer s.detectionMutex.Unlock()

	if s.environmentChecked {
		return nil // Already checked
	}

	// Check if this is a CDB-capable database
	var isCDB int64
	err := s.db.QueryRowContext(ctx, queries.CheckCDBFeatureSQL).Scan(&isCDB)
	if err != nil {
		if errors.IsPermanentError(err) {
			// Likely an older Oracle version that doesn't support CDB
			s.logger.Info("Database does not support CDB features",
				zap.String("instance", s.instanceName),
				zap.Error(err))
			cdbCapable := false
			pdbCapable := false
			s.isCDBCapable = &cdbCapable
			s.isPDBCapable = &pdbCapable
			s.environmentChecked = true
			return nil
		}
		return errors.NewQueryError("cdb_capability_check", queries.CheckCDBFeatureSQL, err,
			map[string]interface{}{"instance": s.instanceName})
	}

	cdbCapable := isCDB == 1
	s.isCDBCapable = &cdbCapable

	// Only check PDB capability if CDB is supported
	if cdbCapable {
		var pdbCount int
		err = s.db.QueryRowContext(ctx, queries.CheckPDBCapabilitySQL).Scan(&pdbCount)
		if err != nil {
			s.logger.Warn("Failed to check PDB capability, assuming not supported",
				zap.String("instance", s.instanceName),
				zap.Error(err))
			pdbCapable := false
			s.isPDBCapable = &pdbCapable
		} else {
			pdbCapable := pdbCount > 0
			s.isPDBCapable = &pdbCapable
		}
	} else {
		pdbCapable := false
		s.isPDBCapable = &pdbCapable
	}

	s.environmentChecked = true

	s.logger.Info("Oracle environment capability detected",
		zap.String("instance", s.instanceName),
		zap.Bool("cdb_capable", *s.isCDBCapable),
		zap.Bool("pdb_capable", *s.isPDBCapable))

	return nil
}

// isCDBSupported returns true if the database supports CDB features
func (s *TablespaceScraper) isCDBSupported() bool {
	s.detectionMutex.RLock()
	defer s.detectionMutex.RUnlock()
	return s.isCDBCapable != nil && *s.isCDBCapable
}

// isPDBSupported returns true if the database supports PDB features
func (s *TablespaceScraper) isPDBSupported() bool {
	s.detectionMutex.RLock()
	defer s.detectionMutex.RUnlock()
	return s.isPDBCapable != nil && *s.isPDBCapable
}

// checkCurrentContext determines which container context we're connected to
func (s *TablespaceScraper) checkCurrentContext(ctx context.Context) error {
	s.detectionMutex.Lock()
	defer s.detectionMutex.Unlock()

	if s.contextChecked {
		return nil
	}

	s.logger.Debug("Checking current Oracle container context",
		zap.String("instance", s.instanceName))

	// Query current container context
	var containerName, containerID string
	err := s.db.QueryRowContext(ctx, queries.CheckCurrentContainerSQL).Scan(&containerName, &containerID)
	if err != nil {
		return errors.NewQueryError("container_context_check", queries.CheckCurrentContainerSQL, err,
			map[string]interface{}{"instance": s.instanceName})
	}

	s.currentContainer = containerName
	s.currentContainerID = containerID
	s.contextChecked = true

	s.logger.Info("Oracle container context detected",
		zap.String("instance", s.instanceName),
		zap.String("container_name", s.currentContainer),
		zap.String("container_id", s.currentContainerID))

	return nil
}

// isConnectedToCDBRoot returns true if connected to CDB$ROOT
func (s *TablespaceScraper) isConnectedToCDBRoot() bool {
	s.detectionMutex.RLock()
	defer s.detectionMutex.RUnlock()
	return s.currentContainer == "CDB$ROOT"
}

// isConnectedToPDB returns true if connected to a specific PDB
func (s *TablespaceScraper) isConnectedToPDB() bool {
	s.detectionMutex.RLock()
	defer s.detectionMutex.RUnlock()
	return s.currentContainer != "" && s.currentContainer != "CDB$ROOT"
}
