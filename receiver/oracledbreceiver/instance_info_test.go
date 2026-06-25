// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

// errQuery is a sentinel error used to simulate a failed DB query.
var errQuery = errors.New("ORA-00942: table or view does not exist")

// versionRow builds the fakeDbClient response for the v$instance version query.
func versionRow(v string) []metricRow {
	return []metricRow{{"VERSION": v}}
}

// cdbRow builds the fakeDbClient response for the v$database CDB/role/open_mode query.
func cdbRow(cdb, role, openMode string) []metricRow {
	return []metricRow{{"CDB": cdb, "DATABASE_ROLE": role, "OPEN_MODE": openMode}}
}

// conTypeRow builds the fakeDbClient response for the USERENV CON_ID query.
// The decode() expression uses AS con_type so the column is keyed by CON_TYPE.
func conTypeRow(t string) []metricRow {
	return []metricRow{{colConType: t}}
}

// conNameRow builds the fakeDbClient response for the USERENV CON_NAME query.
func conNameRow(name string) []metricRow {
	return []metricRow{{colConName: name}}
}

// rdsRow builds the fakeDbClient response for the RDS datafile path probe.
func rdsRow(path string) []metricRow {
	return []metricRow{{"PATH": path}}
}

// ociRow builds the fakeDbClient response for the OCI cloud_identity probe.
func ociRow() []metricRow {
	return []metricRow{{"1": "1"}}
}

// cdbServicesRow builds the fakeDbClient response for the OCI cdb_services confirmation probe.
func cdbServicesRow() []metricRow {
	return []metricRow{{"1": "1"}}
}

// noopClient returns a fakeDbClient that should never be called.
// Use it for detection steps that must not run in a given test.
func noopClient(t *testing.T) dbClient {
	t.Helper()
	return &fakeDbClient{
		Err: errors.New("this client should not have been called"),
	}
}

// errClient returns a fakeDbClient that always returns an error.
func errClient() dbClient {
	return &fakeDbClient{Err: errQuery}
}

// rowClient returns a fakeDbClient that returns the given rows once.
func rowClient(rows []metricRow) dbClient {
	return &fakeDbClient{Responses: [][]metricRow{rows}}
}

// emptyClient returns a fakeDbClient that returns no rows and no error.
func emptyClient() dbClient {
	return &fakeDbClient{Responses: [][]metricRow{{}}}
}

// -- majorVersion unit tests --------------------------------------------------

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected int
	}{
		{name: "empty version returns -1", version: "", expected: -1},
		{name: "19.0.0.0.0", version: "19.0.0.0.0", expected: 19},
		{name: "12.2.0.1.0", version: "12.2.0.1.0", expected: 12},
		{name: "11.2.0.4.0", version: "11.2.0.4.0", expected: 11},
		{name: "21.0.0.0.0", version: "21.0.0.0.0", expected: 21},
		{name: "major-only version string", version: "19", expected: 19},
		{name: "malformed version returns -1", version: "not-a-version", expected: -1},
		{name: "version with only dot returns -1", version: ".", expected: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, majorVersion(tt.version))
		})
	}
}

// -- detectInstanceInfo tests -------------------------------------------------

func TestDetectInstanceInfo_VersionQueryFails(t *testing.T) {
	// Version query fails: all fields stay at zero, detection stops.
	core, logs := observer.New(zapcore.WarnLevel)

	info := detectInstanceInfo(t.Context(),
		errClient(),
		noopClient(t), noopClient(t), noopClient(t), noopClient(t), noopClient(t), noopClient(t),
		zap.New(core),
	)

	assert.Empty(t, info.dbVersion)
	assert.False(t, info.isCDB)
	assert.False(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to detect Oracle version; oracle.db.version attribute will not be set").Len())
}

func TestDetectInstanceInfo_Pre12c(t *testing.T) {
	// Oracle 11g: version set; multitenant and hosting type detection skipped.
	core, logs := observer.New(zapcore.InfoLevel)

	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("11.2.0.4.0")),
		noopClient(t), noopClient(t), noopClient(t),
		noopClient(t), noopClient(t), noopClient(t),
		zap.New(core),
	)

	assert.Equal(t, "11.2.0.4.0", info.dbVersion)
	assert.False(t, info.isCDB)
	assert.False(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Empty(t, info.hostingType)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: Oracle version is pre-12c; multitenant detection skipped").Len())
}

func TestDetectInstanceInfo_NonCDB19c(t *testing.T) {
	// Oracle 19c non-CDB: role and open_mode populated, hosting type detection runs.
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("NO", "PRIMARY", "READ WRITE")),
		noopClient(t), noopClient(t),
		emptyClient(), emptyClient(), emptyClient(),
		zap.NewNop(),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.False(t, info.isCDB)
	assert.Equal(t, "PRIMARY", info.databaseRole)
	assert.Equal(t, "READ WRITE", info.openMode)
	assert.Equal(t, hostingTypeSelfManaged, info.hostingType)
}

func TestDetectInstanceInfo_NonCDB12c(t *testing.T) {
	// Oracle 12c non-CDB: hosting type detection skipped (requires ≥19c).
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("12.2.0.1.0")),
		rowClient(cdbRow("NO", "PRIMARY", "READ WRITE")),
		noopClient(t), noopClient(t),
		noopClient(t), noopClient(t), noopClient(t),
		zap.NewNop(),
	)

	assert.Equal(t, "12.2.0.1.0", info.dbVersion)
	assert.False(t, info.isCDB)
	assert.Equal(t, "PRIMARY", info.databaseRole)
	assert.Equal(t, "READ WRITE", info.openMode)
	assert.Empty(t, info.hostingType)
}

func TestDetectInstanceInfo_CDBQueryFails(t *testing.T) {
	// v$database query fails: isCDB stays false, hosting type not set.
	core, logs := observer.New(zapcore.WarnLevel)

	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		errClient(),
		noopClient(t), noopClient(t),
		noopClient(t), noopClient(t), noopClient(t),
		zap.New(core),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.False(t, info.isCDB)
	assert.False(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Empty(t, info.hostingType)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to detect CDB status; assuming non-CDB").Len())
}

func TestDetectInstanceInfo_CDBRootConnection(t *testing.T) {
	// CDB root connection: connectedToPDB=false, OCI probe skipped.
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("CDB")),
		noopClient(t),
		emptyClient(), emptyClient(), emptyClient(),
		zap.NewNop(),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.True(t, info.isCDB)
	assert.Equal(t, "PRIMARY", info.databaseRole)
	assert.Equal(t, "READ WRITE", info.openMode)
	assert.False(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Equal(t, hostingTypeSelfManaged, info.hostingType)
}

func TestDetectInstanceInfo_ConnTypeQueryFails(t *testing.T) {
	// USERENV CON_ID query fails: connectedToPDB stays false, conNameClient not called.
	core, logs := observer.New(zapcore.WarnLevel)

	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		errClient(),
		noopClient(t),
		noopClient(t), noopClient(t), noopClient(t),
		zap.New(core),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.True(t, info.isCDB)
	assert.False(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to detect connection type (CDB root vs PDB)").Len())
}

func TestDetectInstanceInfo_PDBConnection(t *testing.T) {
	// All steps succeed: all fields populated.
	core, logs := observer.New(zapcore.InfoLevel)

	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("PDB")),
		rowClient(conNameRow("MYPDB")),
		emptyClient(), emptyClient(), emptyClient(),
		zap.New(core),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.Equal(t, "PRIMARY", info.databaseRole)
	assert.Equal(t, "READ WRITE", info.openMode)
	assert.True(t, info.isCDB)
	assert.True(t, info.connectedToPDB)
	assert.Equal(t, "MYPDB", info.pdbName)
	assert.Equal(t, 1, logs.FilterField(zap.String("pdb_name", "MYPDB")).Len())
}

func TestDetectInstanceInfo_PDBNameQueryFails(t *testing.T) {
	// CON_NAME query fails: connectedToPDB=true but pdbName stays empty.
	core, logs := observer.New(zapcore.WarnLevel)

	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("PDB")),
		errClient(),
		emptyClient(), emptyClient(), emptyClient(),
		zap.New(core),
	)

	assert.Equal(t, "19.0.0.0.0", info.dbVersion)
	assert.True(t, info.isCDB)
	assert.True(t, info.connectedToPDB)
	assert.Empty(t, info.pdbName)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to detect PDB name").Len())
}

func TestDetectInstanceInfo_CDBFlagCaseInsensitive(t *testing.T) {
	// Oracle may return "YES", "Yes", or "yes" — all must set isCDB=true.
	for _, cdbVal := range []string{"YES", "Yes", "yes"} {
		t.Run("cdb="+cdbVal, func(t *testing.T) {
			info := detectInstanceInfo(t.Context(),
				rowClient(versionRow("19.0.0.0.0")),
				rowClient(cdbRow(cdbVal, "PRIMARY", "READ WRITE")),
				rowClient(conTypeRow("CDB")),
				noopClient(t),
				emptyClient(), emptyClient(), emptyClient(),
				zap.NewNop(),
			)
			assert.True(t, info.isCDB, "expected isCDB=true for cdb=%q", cdbVal)
		})
	}
}

func TestDetectInstanceInfo_Oracle12c(t *testing.T) {
	// Oracle 12c: multitenant detection runs, but hosting type skipped (requires ≥19c).
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("12.2.0.1.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("PDB")),
		rowClient(conNameRow("SALESPDB")),
		noopClient(t), noopClient(t), noopClient(t),
		zap.NewNop(),
	)

	assert.Equal(t, "12.2.0.1.0", info.dbVersion)
	assert.True(t, info.isCDB)
	assert.True(t, info.connectedToPDB)
	assert.Equal(t, "SALESPDB", info.pdbName)
	assert.Empty(t, info.hostingType)
}

// -- detectHostingType tests --------------------------------------------------

func TestDetectHostingType_SelfManaged(t *testing.T) {
	result := detectHostingType(t.Context(),
		emptyClient(), emptyClient(), emptyClient(),
		false, zap.NewNop(),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
}

func TestDetectHostingType_RDS(t *testing.T) {
	result := detectHostingType(t.Context(),
		rowClient(rdsRow("/rdsdbdata")),
		emptyClient(), emptyClient(),
		false, zap.NewNop(),
	)
	assert.Equal(t, hostingTypeRDS, result)
}

func TestDetectHostingType_RDSQueryFails(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	result := detectHostingType(t.Context(),
		errClient(), emptyClient(), emptyClient(),
		false, zap.New(core),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to probe RDS hosting; hosting type detection may be inaccurate").Len())
}

func TestDetectHostingType_OCI(t *testing.T) {
	// OCI cloud_identity match + cdb_services confirmation → OCI.
	result := detectHostingType(t.Context(),
		emptyClient(),
		rowClient(ociRow()),
		rowClient(cdbServicesRow()),
		true, zap.NewNop(),
	)
	assert.Equal(t, hostingTypeOCI, result)
}

func TestDetectHostingType_OCISkippedWhenNotConnectedToPDB(t *testing.T) {
	result := detectHostingType(t.Context(),
		emptyClient(),
		&fakeDbClient{Err: errors.New("oci client must not be called")},
		&fakeDbClient{Err: errors.New("cdb_services client must not be called")},
		false, zap.NewNop(),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
}

func TestDetectHostingType_OCIFirstQueryFails(t *testing.T) {
	// v$pdbs errors → self-managed, cdb_services not called.
	core, logs := observer.New(zapcore.WarnLevel)
	result := detectHostingType(t.Context(),
		emptyClient(),
		errClient(),
		&fakeDbClient{Err: errors.New("cdb_services client must not be called")},
		true, zap.New(core),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to probe OCI hosting via v$pdbs; hosting type detection may be inaccurate").Len())
}

func TestDetectHostingType_OCIFirstMatchButCDBServicesFails(t *testing.T) {
	// v$pdbs matches but cdb_services errors → self-managed (both must confirm).
	core, logs := observer.New(zapcore.WarnLevel)
	result := detectHostingType(t.Context(),
		emptyClient(),
		rowClient(ociRow()),
		errClient(),
		true, zap.New(core),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
	assert.Equal(t, 1, logs.FilterMessage("oracledbreceiver: failed to probe OCI hosting via cdb_services; hosting type detection may be inaccurate").Len())
}

func TestDetectHostingType_OCIFirstMatchButCDBServicesEmpty(t *testing.T) {
	// v$pdbs matches but cdb_services returns no rows → self-managed (both must confirm).
	result := detectHostingType(t.Context(),
		emptyClient(),
		rowClient(ociRow()),
		emptyClient(),
		true, zap.NewNop(),
	)
	assert.Equal(t, hostingTypeSelfManaged, result)
}

func TestDetectInstanceInfo_HostingTypeRDS(t *testing.T) {
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("NO", "PRIMARY", "READ WRITE")),
		noopClient(t), noopClient(t),
		rowClient(rdsRow("/rdsdbdata")),
		emptyClient(), emptyClient(),
		zap.NewNop(),
	)
	assert.Equal(t, hostingTypeRDS, info.hostingType)
}

func TestDetectInstanceInfo_HostingTypeOCI(t *testing.T) {
	// 19c CDB connected to PDB on OCI (both v$pdbs and cdb_services confirm).
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("PDB")),
		rowClient(conNameRow("MYPDB")),
		emptyClient(),
		rowClient(ociRow()),
		rowClient(cdbServicesRow()),
		zap.NewNop(),
	)
	assert.Equal(t, hostingTypeOCI, info.hostingType)
}

func TestDetectInstanceInfo_HostingTypeOCISkippedForCDBRoot(t *testing.T) {
	// CDB root: OCI probe skipped because connectedToPDB=false.
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("CDB")),
		noopClient(t),
		emptyClient(),
		&fakeDbClient{Err: errors.New("oci client must not be called")},
		&fakeDbClient{Err: errors.New("cdb_services client must not be called")},
		zap.NewNop(),
	)
	assert.Equal(t, hostingTypeSelfManaged, info.hostingType)
}

func TestDetectInstanceInfo_HostingTypeOCISkippedFor12c(t *testing.T) {
	// 12c: hosting type detection skipped entirely (requires ≥19c).
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("12.2.0.1.0")),
		rowClient(cdbRow("YES", "PRIMARY", "READ WRITE")),
		rowClient(conTypeRow("PDB")),
		rowClient(conNameRow("SALESPDB")),
		noopClient(t), noopClient(t), noopClient(t),
		zap.NewNop(),
	)
	assert.Empty(t, info.hostingType)
}

func TestDetectInstanceInfo_PhysicalStandby(t *testing.T) {
	// Data Guard standby: role is PHYSICAL STANDBY, open_mode is READ ONLY WITH APPLY.
	info := detectInstanceInfo(t.Context(),
		rowClient(versionRow("19.0.0.0.0")),
		rowClient(cdbRow("NO", "PHYSICAL STANDBY", "READ ONLY WITH APPLY")),
		noopClient(t), noopClient(t),
		emptyClient(), emptyClient(), emptyClient(),
		zap.NewNop(),
	)

	assert.Equal(t, "PHYSICAL STANDBY", info.databaseRole)
	assert.Equal(t, "READ ONLY WITH APPLY", info.openMode)
}

// -- setupResourceBuilder tests -----------------------------------------------

func TestSetupResourceBuilder_NoPDB(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	scrpr := oracleScraper{
		mb:                   metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		metricsBuilderConfig: cfg,
		instanceName:         "myinstance",
		hostName:             "myhost",
		instanceInfo:         oracleInstanceInfo{dbVersion: "19.0.0.0.0", isCDB: false},
	}

	res := scrpr.setupResourceBuilder(scrpr.mb.NewResourceBuilder()).Emit()

	_, hasPDB := res.Attributes().Get("oracle.db.pdb")
	assert.False(t, hasPDB)

	name, _ := res.Attributes().Get("oracledb.instance.name")
	assert.Equal(t, "myinstance", name.Str())
	host, _ := res.Attributes().Get("host.name")
	assert.Equal(t, "myhost", host.Str())
	version, _ := res.Attributes().Get("oracle.db.version")
	assert.Equal(t, "19.0.0.0.0", version.Str())
}

func TestSetupResourceBuilder_AllMetadataFields(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	scrpr := oracleScraper{
		mb:                   metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		metricsBuilderConfig: cfg,
		instanceName:         "myinstance",
		hostName:             "myhost",
		instanceInfo: oracleInstanceInfo{
			dbVersion:    "19.0.0.0.0",
			databaseRole: "PRIMARY",
			openMode:     "READ WRITE",
			hostingType:  hostingTypeSelfManaged,
		},
	}

	res := scrpr.setupResourceBuilder(scrpr.mb.NewResourceBuilder()).Emit()

	version, ok := res.Attributes().Get("oracle.db.version")
	require.True(t, ok)
	assert.Equal(t, "19.0.0.0.0", version.Str())

	role, ok := res.Attributes().Get("oracle.db.role")
	require.True(t, ok)
	assert.Equal(t, "PRIMARY", role.Str())

	openMode, ok := res.Attributes().Get("oracle.db.open_mode")
	require.True(t, ok)
	assert.Equal(t, "READ WRITE", openMode.Str())

	hostingType, ok := res.Attributes().Get("oracle.db.hosting_type")
	require.True(t, ok)
	assert.Equal(t, hostingTypeSelfManaged, hostingType.Str())
}

func TestSetupResourceBuilder_EmptyMetadataFieldsNotEmitted(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	scrpr := oracleScraper{
		mb:                   metadata.NewMetricsBuilder(cfg, receivertest.NewNopSettings(metadata.Type)),
		metricsBuilderConfig: cfg,
		instanceInfo:         oracleInstanceInfo{},
	}

	res := scrpr.setupResourceBuilder(scrpr.mb.NewResourceBuilder()).Emit()

	for _, attr := range []string{"oracle.db.version", "oracle.db.role", "oracle.db.open_mode", "oracle.db.hosting_type"} {
		_, exists := res.Attributes().Get(attr)
		assert.False(t, exists, "attribute %q should not be emitted when empty", attr)
	}
}
