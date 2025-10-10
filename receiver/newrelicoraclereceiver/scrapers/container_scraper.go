// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// ContainerScraper handles Oracle CDB/PDB container metrics
type ContainerScraper struct {
	db                 *sql.DB
	mb                 *metadata.MetricsBuilder
	logger             *zap.Logger
	instanceName       string
	config             metadata.MetricsBuilderConfig
	isCDBCapable       *bool  // Cache for CDB capability check
	isPDBCapable       *bool  // Cache for PDB capability check
	environmentChecked bool   // Track if environment has been checked
	currentContainer   string // Current container context (CDB$ROOT, FREEPDB1, etc.)
	currentContainerID string // Current container ID (1, 3, etc.)
	contextChecked     bool   // Track if context has been checked
}

// NewContainerScraper creates a new Container scraper
func NewContainerScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *ContainerScraper {
	return &ContainerScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeContainerMetrics collects Oracle CDB/PDB container metrics
func (s *ContainerScraper) ScrapeContainerMetrics(ctx context.Context) []error {
	var errors []error

	s.logger.Debug("Scraping Oracle CDB/PDB container metrics")

	// Check environment capability first
	if err := s.checkEnvironmentCapability(ctx); err != nil {
		s.logger.Error("Failed to check Oracle environment capabilities", zap.Error(err))
		return []error{err}
	}

	// Skip if CDB/PDB features are not supported
	if !s.isCDBSupported() {
		s.logger.Debug("CDB features not supported, skipping container metrics")
		return errors
	}

	// Check current container context
	if err := s.checkCurrentContext(ctx); err != nil {
		s.logger.Error("Failed to check Oracle container context", zap.Error(err))
		return []error{err}
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	// Scrape container status metrics (only from CDB$ROOT)
	if s.isConnectedToCDBRoot() {
		errors = append(errors, s.scrapeContainerStatus(ctx, now)...)
	} else {
		s.logger.Debug("Not connected to CDB$ROOT, skipping container status metrics",
			zap.String("current_container", s.currentContainer))
	}

	// Scrape PDB status metrics (only if PDB is supported and connected to CDB$ROOT)
	if s.isPDBSupported() && s.isConnectedToCDBRoot() {
		errors = append(errors, s.scrapePDBStatus(ctx, now)...)
	} else {
		s.logger.Debug("PDB features not supported or not in CDB$ROOT, skipping PDB metrics",
			zap.String("current_container", s.currentContainer))
	}

	// Scrape CDB tablespace usage (only from CDB$ROOT)
	if s.isConnectedToCDBRoot() {
		errors = append(errors, s.scrapeCDBTablespaceUsage(ctx, now)...)
	} else {
		s.logger.Debug("Not connected to CDB$ROOT, skipping CDB tablespace metrics",
			zap.String("current_container", s.currentContainer))
	}

	// Scrape CDB data files (only from CDB$ROOT)
	if s.isConnectedToCDBRoot() {
		errors = append(errors, s.scrapeCDBDataFiles(ctx, now)...)
	} else {
		s.logger.Debug("Not connected to CDB$ROOT, skipping CDB data file metrics",
			zap.String("current_container", s.currentContainer))
	}

	// Scrape CDB services (only from CDB$ROOT)
	if s.isConnectedToCDBRoot() {
		errors = append(errors, s.scrapeCDBServices(ctx, now)...)
	} else {
		s.logger.Debug("Not connected to CDB$ROOT, skipping CDB service metrics",
			zap.String("current_container", s.currentContainer))
	}

	return errors
}

// scrapeContainerStatus scrapes container status from GV$CONTAINERS
func (s *ContainerScraper) scrapeContainerStatus(ctx context.Context, now pcommon.Timestamp) []error {
	var errs []error

	// Check if this metric is enabled in config
	if !s.config.Metrics.NewrelicoracledbContainerStatus.Enabled &&
		!s.config.Metrics.NewrelicoracledbContainerRestricted.Enabled {
		s.logger.Debug("Container status metrics disabled, skipping")
		return errs
	}

	s.logger.Debug("Scraping Oracle container status",
		zap.String("sql", errors.FormatQueryForLogging(queries.ContainerStatusSQL)))

	rows, err := s.db.QueryContext(ctx, queries.ContainerStatusSQL)
	if err != nil {
		scraperErr := errors.NewQueryError(
			"container_status_query",
			queries.ContainerStatusSQL,
			err,
			map[string]interface{}{
				"instance":  s.instanceName,
				"retryable": errors.IsRetryableError(err),
				"permanent": errors.IsPermanentError(err),
			},
		)

		if errors.IsPermanentError(err) {
			s.logger.Error("Permanent error executing container status query", zap.Error(scraperErr))
		} else {
			s.logger.Warn("Error executing container status query", zap.Error(scraperErr))
		}
		return []error{scraperErr}
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.logger.Warn("Failed to close container status query rows", zap.Error(closeErr))
		}
	}()

	rowCount := 0
	for rows.Next() {
		// Check for context cancellation during iteration
		select {
		case <-ctx.Done():
			return []error{errors.NewScraperError("container_status_iteration", ctx.Err(),
				map[string]interface{}{"rows_processed": rowCount})}
		default:
		}

		var conID sql.NullInt64
		var containerName sql.NullString
		var openMode sql.NullString
		var restricted sql.NullString
		var openTime sql.NullTime

		if err := rows.Scan(&conID, &containerName, &openMode, &restricted, &openTime); err != nil {
			scanErr := errors.NewScraperError("container_status_scan", err,
				map[string]interface{}{
					"row_number": rowCount,
					"instance":   s.instanceName,
				})
			s.logger.Error("Error scanning container status row", zap.Error(scanErr))
			errs = append(errs, scanErr)
			continue
		}

		rowCount++

		// Validate required fields
		if !conID.Valid {
			s.logger.Debug("Skipping container status row with invalid CON_ID", zap.Int("row_number", rowCount))
			continue
		}
		if !containerName.Valid {
			s.logger.Debug("Skipping container status row with invalid container name",
				zap.Int64("con_id", conID.Int64), zap.Int("row_number", rowCount))
			continue
		}

		conIDStr := strconv.FormatInt(conID.Int64, 10)
		containerNameStr := containerName.String
		openModeStr := ""
		restrictedStr := ""

		if openMode.Valid {
			openModeStr = openMode.String
		}
		if restricted.Valid {
			restrictedStr = restricted.String
		}

		// Record metrics only if enabled
		if s.config.Metrics.NewrelicoracledbContainerStatus.Enabled {
			// Container status metric (1=READ WRITE, 0=other)
			var statusValue int64 = 0
			if strings.ToUpper(openModeStr) == "READ WRITE" {
				statusValue = 1
			}
			s.mb.RecordNewrelicoracledbContainerStatusDataPoint(now, statusValue, s.instanceName, conIDStr, containerNameStr, openModeStr)
		}

		if s.config.Metrics.NewrelicoracledbContainerRestricted.Enabled {
			// Container restricted metric (1=YES, 0=NO)
			var restrictedValue int64 = 0
			if strings.ToUpper(restrictedStr) == "YES" {
				restrictedValue = 1
			}
			s.mb.RecordNewrelicoracledbContainerRestrictedDataPoint(now, restrictedValue, s.instanceName, conIDStr, containerNameStr, restrictedStr)
		}

		s.logger.Debug("Processed container status",
			zap.String("con_id", conIDStr),
			zap.String("container_name", containerNameStr),
			zap.String("open_mode", openModeStr),
			zap.String("restricted", restrictedStr))
	}

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		iterErr := errors.NewScraperError("container_status_iteration", err,
			map[string]interface{}{
				"rows_processed": rowCount,
				"instance":       s.instanceName,
			})
		s.logger.Error("Error during container status rows iteration", zap.Error(iterErr))
		errs = append(errs, iterErr)
	}

	s.logger.Debug("Completed container status scraping",
		zap.Int("rows_processed", rowCount),
		zap.Int("errors", len(errs)))

	return errs
}

// scrapePDBStatus scrapes PDB status from GV$PDBS
func (s *ContainerScraper) scrapePDBStatus(ctx context.Context, now pcommon.Timestamp) []error {
	rows, err := s.db.QueryContext(ctx, queries.PDBStatusSQL)
	if err != nil {
		s.logger.Error("Failed to execute PDB status query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var conID sql.NullInt64
		var pdbName sql.NullString
		var status sql.NullString
		var creationSCN sql.NullInt64
		var openMode sql.NullString
		var restricted sql.NullString
		var openTime sql.NullTime
		var totalSize sql.NullInt64

		if err := rows.Scan(&conID, &pdbName, &status, &creationSCN, &openMode, &restricted, &openTime, &totalSize); err != nil {
			s.logger.Error("Error scanning PDB status row", zap.Error(err))
			continue
		}

		if !conID.Valid || !pdbName.Valid {
			continue
		}

		conIDStr := strconv.FormatInt(conID.Int64, 10)
		pdbNameStr := pdbName.String
		statusStr := ""
		openModeStr := ""

		if status.Valid {
			statusStr = status.String
		}
		if openMode.Valid {
			openModeStr = openMode.String
		}

		// PDB status metric (1=NORMAL, 0=other)
		var statusValue int64 = 0
		if strings.ToUpper(statusStr) == "NORMAL" {
			statusValue = 1
		}
		s.mb.RecordNewrelicoracledbPdbStatusDataPoint(now, statusValue, s.instanceName, conIDStr, pdbNameStr, statusStr)

		// PDB open mode metric (1=READ WRITE, 0=other)
		var openModeValue int64 = 0
		if strings.ToUpper(openModeStr) == "READ WRITE" {
			openModeValue = 1
		}
		s.mb.RecordNewrelicoracledbPdbOpenModeDataPoint(now, openModeValue, s.instanceName, conIDStr, pdbNameStr, openModeStr)

		// PDB total size metric
		if totalSize.Valid {
			s.mb.RecordNewrelicoracledbPdbTotalSizeBytesDataPoint(now, totalSize.Int64, s.instanceName, conIDStr, pdbNameStr)
		}

		s.logger.Debug("Processed PDB status",
			zap.String("con_id", conIDStr),
			zap.String("pdb_name", pdbNameStr),
			zap.String("status", statusStr),
			zap.String("open_mode", openModeStr))
	}

	return nil
}

// scrapeCDBTablespaceUsage scrapes CDB tablespace usage from CDB_TABLESPACE_USAGE_METRICS
func (s *ContainerScraper) scrapeCDBTablespaceUsage(ctx context.Context, now pcommon.Timestamp) []error {
	rows, err := s.db.QueryContext(ctx, queries.CDBTablespaceUsageSQL)
	if err != nil {
		s.logger.Error("Failed to execute CDB tablespace usage query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var conID sql.NullInt64
		var tablespaceName sql.NullString
		var usedBytes sql.NullInt64
		var totalBytes sql.NullInt64
		var usedPercent sql.NullFloat64

		if err := rows.Scan(&conID, &tablespaceName, &usedBytes, &totalBytes, &usedPercent); err != nil {
			s.logger.Error("Error scanning CDB tablespace usage row", zap.Error(err))
			continue
		}

		if !conID.Valid || !tablespaceName.Valid {
			continue
		}

		conIDStr := strconv.FormatInt(conID.Int64, 10)
		tablespaceNameStr := tablespaceName.String

		// Record tablespace usage metrics with container tagging
		if usedBytes.Valid {
			s.mb.RecordNewrelicoracledbTablespaceUsedBytesDataPoint(now, usedBytes.Int64, s.instanceName, conIDStr, tablespaceNameStr)
		}

		if totalBytes.Valid {
			s.mb.RecordNewrelicoracledbTablespaceTotalBytesDataPoint(now, totalBytes.Int64, s.instanceName, conIDStr, tablespaceNameStr)
		}

		if usedPercent.Valid {
			s.mb.RecordNewrelicoracledbTablespaceUsedPercentDataPoint(now, usedPercent.Float64, s.instanceName, conIDStr, tablespaceNameStr)
		}

		s.logger.Debug("Processed CDB tablespace usage",
			zap.String("con_id", conIDStr),
			zap.String("tablespace_name", tablespaceNameStr),
			zap.Int64("used_bytes", usedBytes.Int64),
			zap.Int64("total_bytes", totalBytes.Int64))
	}

	return nil
}

// scrapeCDBDataFiles scrapes CDB data files from CDB_DATA_FILES
func (s *ContainerScraper) scrapeCDBDataFiles(ctx context.Context, now pcommon.Timestamp) []error {
	rows, err := s.db.QueryContext(ctx, queries.CDBDataFilesSQL)
	if err != nil {
		s.logger.Error("Failed to execute CDB data files query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var conID sql.NullInt64
		var fileName sql.NullString
		var tablespaceName sql.NullString
		var bytes sql.NullInt64
		var status sql.NullString
		var autoextensible sql.NullString
		var maxBytes sql.NullInt64
		var userBytes sql.NullInt64

		if err := rows.Scan(&conID, &fileName, &tablespaceName, &bytes, &status, &autoextensible, &maxBytes, &userBytes); err != nil {
			s.logger.Error("Error scanning CDB data files row", zap.Error(err))
			continue
		}

		if !conID.Valid || !fileName.Valid || !tablespaceName.Valid {
			continue
		}

		conIDStr := strconv.FormatInt(conID.Int64, 10)
		fileNameStr := fileName.String
		tablespaceNameStr := tablespaceName.String
		autoextensibleStr := ""

		if autoextensible.Valid {
			autoextensibleStr = autoextensible.String
		}

		// Record data file size
		if bytes.Valid {
			s.mb.RecordNewrelicoracledbDatafileSizeBytesDataPoint(now, bytes.Int64, s.instanceName, conIDStr, tablespaceNameStr, fileNameStr)
		}

		// Record user bytes
		if userBytes.Valid {
			s.mb.RecordNewrelicoracledbDatafileUsedBytesDataPoint(now, userBytes.Int64, s.instanceName, conIDStr, tablespaceNameStr, fileNameStr)
		}

		// Record autoextensible status (1=YES, 0=NO)
		var autoextensibleValue int64 = 0
		if strings.ToUpper(autoextensibleStr) == "YES" {
			autoextensibleValue = 1
		}
		s.mb.RecordNewrelicoracledbDatafileAutoextensibleDataPoint(now, autoextensibleValue, s.instanceName, conIDStr, tablespaceNameStr, fileNameStr, autoextensibleStr)

		s.logger.Debug("Processed CDB data file",
			zap.String("con_id", conIDStr),
			zap.String("tablespace_name", tablespaceNameStr),
			zap.String("file_name", fileNameStr),
			zap.Int64("bytes", bytes.Int64))
	}

	return nil
}

// scrapeCDBServices scrapes CDB services from CDB_SERVICES
func (s *ContainerScraper) scrapeCDBServices(ctx context.Context, now pcommon.Timestamp) []error {
	rows, err := s.db.QueryContext(ctx, queries.CDBServicesSQL)
	if err != nil {
		s.logger.Error("Failed to execute CDB services query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	serviceCount := make(map[string]int64)

	for rows.Next() {
		var conID sql.NullInt64
		var serviceName sql.NullString
		var networkName sql.NullString
		var creationDate sql.NullTime
		var pdb sql.NullString
		var enabled sql.NullString

		if err := rows.Scan(&conID, &serviceName, &networkName, &creationDate, &pdb, &enabled); err != nil {
			s.logger.Error("Error scanning CDB services row", zap.Error(err))
			continue
		}

		if !conID.Valid || !serviceName.Valid {
			continue
		}

		conIDStr := strconv.FormatInt(conID.Int64, 10)
		serviceNameStr := serviceName.String

		// Count services per container
		serviceCount[conIDStr]++

		// Record service status (1=enabled if enabled='YES', 0=disabled)
		var serviceStatus int64 = 0
		if enabled.Valid && strings.ToUpper(enabled.String) == "YES" {
			serviceStatus = 1
		}
		s.mb.RecordNewrelicoracledbServiceStatusDataPoint(now, serviceStatus, s.instanceName, conIDStr, serviceNameStr)

		s.logger.Debug("Processed CDB service",
			zap.String("con_id", conIDStr),
			zap.String("service_name", serviceNameStr),
			zap.String("pdb", pdb.String),
			zap.String("enabled", enabled.String),
			zap.Int64("status", serviceStatus))
	}

	// Record service count per container
	for conIDStr, count := range serviceCount {
		s.mb.RecordNewrelicoracledbServiceCountDataPoint(now, count, s.instanceName, conIDStr)
	}

	return nil
}

// checkEnvironmentCapability checks if the Oracle database supports CDB/PDB features
func (s *ContainerScraper) checkEnvironmentCapability(ctx context.Context) error {
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

	// Check if PDB views are available
	if cdbCapable {
		var pdbCount int64
		err = s.db.QueryRowContext(ctx, queries.CheckPDBCapabilitySQL).Scan(&pdbCount)
		if err != nil {
			return errors.NewQueryError("pdb_capability_check", queries.CheckPDBCapabilitySQL, err,
				map[string]interface{}{"instance": s.instanceName})
		}
		pdbCapable := pdbCount > 0
		s.isPDBCapable = &pdbCapable
	} else {
		pdbCapable := false
		s.isPDBCapable = &pdbCapable
	}

	s.environmentChecked = true
	s.logger.Info("Detected Oracle environment capabilities",
		zap.String("instance", s.instanceName),
		zap.Bool("cdb_capable", *s.isCDBCapable),
		zap.Bool("pdb_capable", *s.isPDBCapable))

	return nil
}

// isCDBSupported returns true if the database supports CDB features
func (s *ContainerScraper) isCDBSupported() bool {
	return s.isCDBCapable != nil && *s.isCDBCapable
}

// isPDBSupported returns true if the database supports PDB features
func (s *ContainerScraper) isPDBSupported() bool {
	return s.isPDBCapable != nil && *s.isPDBCapable
}

// checkCurrentContext determines which container context we're connected to
func (s *ContainerScraper) checkCurrentContext(ctx context.Context) error {
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

	s.logger.Info("Detected Oracle container context",
		zap.String("instance", s.instanceName),
		zap.String("container_name", s.currentContainer),
		zap.String("container_id", s.currentContainerID))

	return nil
}

// isConnectedToCDBRoot returns true if connected to CDB$ROOT
func (s *ContainerScraper) isConnectedToCDBRoot() bool {
	return s.currentContainer == "CDB$ROOT"
}

// isConnectedToPDB returns true if connected to a specific PDB
func (s *ContainerScraper) isConnectedToPDB() bool {
	return s.currentContainer != "" && s.currentContainer != "CDB$ROOT"
}
