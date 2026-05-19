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

// isVersionGTE reports whether the detected Oracle major version is >= minMajor.
func (info oracleInstanceInfo) isVersionGTE(minMajor int) bool {
	if info.dbVersion == "" {
		return false
	}
	major, err := strconv.Atoi(strings.SplitN(info.dbVersion, ".", 2)[0])
	if err != nil {
		return false
	}
	return major >= minMajor
}

const (
	// minMultitenantVersion is the first Oracle version that supports CDB/PDB.
	minMultitenantVersion = 12
	// minHostingDetectionVersion is the first Oracle version where RDS/OCI probes are reliable.
	minHostingDetectionVersion = 19
	instanceInfoDetectTimeout  = 5 * time.Second

	instanceVersionSQL        = "SELECT version FROM v$instance"
	instanceCDBSQL            = "SELECT cdb, database_role, open_mode FROM v$database"
	instanceConTypeSQL        = "SELECT decode(sys_context('USERENV','CON_ID'),1,'CDB','PDB') FROM dual"
	instanceConNameSQL        = "SELECT sys_context('USERENV','CON_NAME') FROM dual"
	instanceRDSSQL            = "SELECT SUBSTR(name,1,10) path FROM v$datafile WHERE rownum = 1"
	instanceOCISQL            = "SELECT 1 FROM v$pdbs WHERE cloud_identity LIKE '%oraclecloud%' AND rownum = 1"
	instanceOCICDBServicesSQL = "SELECT 1 FROM cdb_services WHERE name LIKE '%oraclecloud%' AND rownum = 1"

	hostingTypeSelfManaged = "self-managed"
	hostingTypeRDS         = "RDS"
	hostingTypeOCI         = "OCI"
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
	info.dbVersion = rows[0]["VERSION"]
	logger.Info("oracledbreceiver: detected Oracle version", zap.String("version", info.dbVersion))

	if !info.isVersionGTE(minMultitenantVersion) {
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
	info.isCDB = strings.EqualFold(rows[0]["CDB"], "YES")
	info.databaseRole = rows[0]["DATABASE_ROLE"]
	info.openMode = rows[0]["OPEN_MODE"]

	if !info.isCDB {
		if info.isVersionGTE(minHostingDetectionVersion) {
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
	// decode() returns an unnamed column; read the first value regardless of key.
	for _, v := range rows[0] {
		info.connectedToPDB = v == "PDB"
		break
	}

	if info.isVersionGTE(minHostingDetectionVersion) {
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
	for _, v := range rows[0] {
		info.pdbName = v
		break
	}
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
		for _, v := range rows[0] {
			if v == "/rdsdbdata" {
				return hostingTypeRDS
			}
			break
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
