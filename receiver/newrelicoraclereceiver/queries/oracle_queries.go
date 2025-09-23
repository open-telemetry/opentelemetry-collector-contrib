// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

// Oracle SQL query for session count metric
const (
	SessionCountSQL = "SELECT COUNT(*) as SESSION_COUNT FROM v$session WHERE type = 'USER'"
)

// Oracle SQL query for tablespace metrics
const (
	TablespaceMetricsSQL = `
		SELECT a.TABLESPACE_NAME,
			a.USED_PERCENT,
			a.USED_SPACE * b.BLOCK_SIZE AS "USED",
			a.TABLESPACE_SIZE * b.BLOCK_SIZE AS "SIZE",
			b.TABLESPACE_OFFLINE AS "OFFLINE"
		FROM DBA_TABLESPACE_USAGE_METRICS a
		JOIN (
			SELECT
				TABLESPACE_NAME,
				BLOCK_SIZE,
				MAX( CASE WHEN status = 'OFFLINE' THEN 1 ELSE 0 END) AS "TABLESPACE_OFFLINE"
			FROM DBA_TABLESPACES
			GROUP BY TABLESPACE_NAME, BLOCK_SIZE
		) b
		ON a.TABLESPACE_NAME = b.TABLESPACE_NAME`

	GlobalNameTablespaceSQL = `
		SELECT
		t1.TABLESPACE_NAME,
		t2.GLOBAL_NAME
		FROM (SELECT TABLESPACE_NAME FROM DBA_TABLESPACES) t1,
		(SELECT GLOBAL_NAME FROM global_name) t2`

	DBIDTablespaceSQL = `
		SELECT
		t1.TABLESPACE_NAME,
		t2.DBID
		FROM (SELECT TABLESPACE_NAME FROM DBA_TABLESPACES) t1,
		(SELECT DBID FROM v$database) t2`

	CDBDatafilesOfflineTablespaceSQL = `
		SELECT
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE', 'SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS CDB_DATAFILES_OFFLINE,
			TABLESPACE_NAME
		FROM dba_data_files
		GROUP BY TABLESPACE_NAME`

	PDBDatafilesOfflineTablespaceSQL = `
		SELECT
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_DATAFILES_OFFLINE,
			a.TABLESPACE_NAME
		FROM cdb_data_files a, cdb_pdbs b
		WHERE a.con_id = b.con_id
		GROUP BY a.TABLESPACE_NAME`

	PDBNonWriteTablespaceSQL = `
		SELECT 
			TABLESPACE_NAME, 
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_NON_WRITE_MODE
		FROM cdb_data_files a, cdb_pdbs b
		WHERE a.con_id = b.con_id
		GROUP BY TABLESPACE_NAME`

	// Core database metrics queries
	LockedAccountsSQL = `
		SELECT
			INST_ID, LOCKED_ACCOUNTS
		FROM
		(	SELECT count(1) AS LOCKED_ACCOUNTS
			FROM
				cdb_users a,
				cdb_pdbs b
			WHERE a.con_id = b.con_id
				AND a.account_status != 'OPEN'
		) l,
		gv$instance i`

	ReadWriteMetricsSQL = `
		SELECT
			INST_ID,
			SUM(PHYRDS) AS PhysicalReads,
			SUM(PHYWRTS) AS PhysicalWrites,
			SUM(PHYBLKRD) AS PhysicalBlockReads,
			SUM(PHYBLKWRT) AS PhysicalBlockWrites,
			SUM(READTIM) * 10 AS ReadTime,
			SUM(WRITETIM) * 10 AS WriteTime
		FROM gv$filestat
		GROUP BY INST_ID`

	PGAMetricsSQL = `
		SELECT INST_ID, NAME, VALUE 
		FROM gv$pgastat 
		WHERE NAME IN ('total PGA inuse', 'total PGA allocated', 'total freeable PGA memory', 'global memory bound')`

	GlobalNameInstanceSQL = `
		SELECT
			t1.INST_ID,
			t2.GLOBAL_NAME
		FROM
			(SELECT INST_ID FROM gv$instance) t1,
			(SELECT GLOBAL_NAME FROM global_name) t2`
)
