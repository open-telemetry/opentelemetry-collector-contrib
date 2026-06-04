// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// oracleInstanceInfo holds Oracle deployment metadata detected once at scraper start time.
type oracleInstanceInfo struct {
	dbVersion      string
	databaseRole   string
	openMode       string
	hostingType    string
	isCDB          bool
	connectedToPDB bool
	pdbName        string
}

// oracleVersionGTE reports whether the given Oracle version string has a major version >= minMajor.
func oracleVersionGTE(version string, minMajor int) bool {
	if version == "" {
		return false
	}
	major, err := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
	if err != nil {
		return false
	}
	return major >= minMajor
}

const (
	hostingTypeOCI         = "OCI"
	hostingTypeRDS         = "RDS"
	hostingTypeSelfManaged = "self-managed"

	instanceInfoDetectTimeout = 5 * time.Second

	instanceCDBSQL            = "SELECT cdb, database_role, open_mode FROM v$database"
	instanceConNameSQL        = "SELECT sys_context('USERENV','CON_NAME') AS con_name FROM dual"
	instanceConTypeSQL        = "SELECT decode(sys_context('USERENV','CON_ID'),1,'CDB','PDB') AS con_type FROM dual"
	instanceOCICDBServicesSQL = "SELECT 1 FROM cdb_services WHERE name LIKE '%oraclecloud%' AND rownum = 1"
	instanceOCISQL            = "SELECT 1 FROM v$pdbs WHERE cloud_identity LIKE '%oraclecloud%' AND rownum = 1"
	instanceRDSSQL            = "SELECT SUBSTR(name,1,10) AS path FROM v$datafile WHERE rownum = 1"
	instanceVersionSQL        = "SELECT version FROM v$instance"

	// minHostingDetectionVersion is the first Oracle version where RDS/OCI probes are reliable.
	minHostingDetectionVersion = 19
	// minMultitenantVersion is the first Oracle version that supports CDB/PDB.
	minMultitenantVersion = 12

	colCDB          = "CDB"
	colConName      = "CON_NAME"
	colConType      = "CON_TYPE"
	colDatabaseRole = "DATABASE_ROLE"
	colOpenMode     = "OPEN_MODE"
	colPath         = "PATH"
	colVersion      = "VERSION"
)

// detectInstanceInfo queries Oracle at startup to populate version, role, and multitenant info.
// Failures are logged at Warn level; affected fields retain their zero value.
func detectInstanceInfo(
	ctx context.Context,
	versionClient dbClient,
	cdbClient dbClient,
	conTypeClient dbClient,
	conNameClient dbClient,
	rdsClient dbClient,
	ociClient dbClient,
	cdbServicesClient dbClient,
	logger *zap.Logger,
) oracleInstanceInfo {
	ctx, cancel := context.WithTimeout(ctx, instanceInfoDetectTimeout)
	defer cancel()

	info := oracleInstanceInfo{}

	rows, err := versionClient.metricRows(ctx)
	if err != nil || len(rows) == 0 {
		logger.Warn("oracledbreceiver: failed to detect Oracle version; oracle.db.version attribute will not be set",
			zap.Error(err))
		return info
	}
	info.dbVersion = rows[0][colVersion]
	logger.Info("oracledbreceiver: detected Oracle version", zap.String("version", info.dbVersion))

	if !oracleVersionGTE(info.dbVersion, minMultitenantVersion) {
		logger.Info("oracledbreceiver: Oracle version is pre-12c; multitenant detection skipped",
			zap.String("version", info.dbVersion))
		return info
	}

	rows, err = cdbClient.metricRows(ctx)
	if err != nil || len(rows) == 0 {
		logger.Warn("oracledbreceiver: failed to detect CDB status; assuming non-CDB",
			zap.Error(err))
		return info
	}
	info.isCDB = strings.EqualFold(rows[0][colCDB], "YES")
	info.databaseRole = rows[0][colDatabaseRole]
	info.openMode = rows[0][colOpenMode]

	if !info.isCDB {
		if oracleVersionGTE(info.dbVersion, minHostingDetectionVersion) {
			info.hostingType = detectHostingType(ctx, rdsClient, ociClient, cdbServicesClient, false, logger)
		}
		return info
	}

	rows, err = conTypeClient.metricRows(ctx)
	if err != nil || len(rows) == 0 {
		logger.Warn("oracledbreceiver: failed to detect connection type (CDB root vs PDB)",
			zap.Error(err))
		return info
	}
	info.connectedToPDB = strings.EqualFold(rows[0][colConType], "PDB")

	if oracleVersionGTE(info.dbVersion, minHostingDetectionVersion) {
		info.hostingType = detectHostingType(ctx, rdsClient, ociClient, cdbServicesClient, info.connectedToPDB, logger)
	}

	if !info.connectedToPDB {
		return info
	}

	rows, err = conNameClient.metricRows(ctx)
	if err != nil || len(rows) == 0 {
		logger.Warn("oracledbreceiver: failed to detect PDB name",
			zap.Error(err))
		return info
	}
	info.pdbName = rows[0][colConName]
	logger.Info("oracledbreceiver: connected to PDB", zap.String("pdb_name", info.pdbName))

	return info
}

// detectHostingType returns hostingTypeRDS, hostingTypeOCI, or hostingTypeSelfManaged.
// RDS is detected via v$datafile path prefix; OCI requires both v$pdbs.cloud_identity
// and cdb_services to match, and only runs when connected to a PDB.
func detectHostingType(
	ctx context.Context,
	rdsClient dbClient,
	ociClient dbClient,
	cdbServicesClient dbClient,
	ociConnectedToPDB bool,
	logger *zap.Logger,
) string {
	rows, err := rdsClient.metricRows(ctx)
	if err != nil {
		logger.Warn("oracledbreceiver: failed to probe RDS hosting; hosting type detection may be inaccurate",
			zap.Error(err))
	} else if len(rows) > 0 {
		if strings.EqualFold(rows[0][colPath], "/rdsdbdata") {
			return hostingTypeRDS
		}
	}

	if ociConnectedToPDB {
		rows, err = ociClient.metricRows(ctx)
		if err != nil {
			logger.Warn("oracledbreceiver: failed to probe OCI hosting via v$pdbs; hosting type detection may be inaccurate",
				zap.Error(err))
		} else if len(rows) > 0 {
			rows, err = cdbServicesClient.metricRows(ctx)
			if err != nil {
				logger.Warn("oracledbreceiver: failed to probe OCI hosting via cdb_services; hosting type detection may be inaccurate",
					zap.Error(err))
			} else if len(rows) > 0 {
				return hostingTypeOCI
			}
		}
	}

	return hostingTypeSelfManaged
}
