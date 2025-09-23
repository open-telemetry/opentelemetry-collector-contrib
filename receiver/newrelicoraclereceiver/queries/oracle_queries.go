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
)
