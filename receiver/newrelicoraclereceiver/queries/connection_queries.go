// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

// Oracle SQL queries for connection statistics

// Core Connection Counts & Status
const (
	// Total Sessions
	TotalSessionsSQL = "SELECT COUNT(*) as TOTAL_SESSIONS FROM V$SESSION"

	// Active Sessions
	ActiveSessionsSQL = "SELECT COUNT(*) as ACTIVE_SESSIONS FROM V$SESSION WHERE STATUS = 'ACTIVE'"

	// Inactive Sessions
	InactiveSessionsSQL = "SELECT COUNT(*) as INACTIVE_SESSIONS FROM V$SESSION WHERE STATUS = 'INACTIVE'"

	// Session Status Breakdown
	SessionStatusSQL = `
		SELECT 
			STATUS,
			COUNT(*) as SESSION_COUNT
		FROM V$SESSION 
		GROUP BY STATUS`

	// Session Type Breakdown
	SessionTypeSQL = `
		SELECT 
			TYPE,
			COUNT(*) as SESSION_COUNT
		FROM V$SESSION 
		GROUP BY TYPE`

	// Logons Statistics
	LogonsStatsSQL = `
		SELECT NAME, VALUE 
		FROM V$SYSSTAT 
		WHERE NAME IN ('logons cumulative', 'logons current')`

	// Session Resource Consumption
	SessionResourceConsumptionSQL = `
		SELECT 
			s.SID,
			s.USERNAME,
			s.STATUS,
			s.PROGRAM,
			s.MACHINE,
			s.OSUSER,
			s.LOGON_TIME,
			s.LAST_CALL_ET,
			ROUND(ss_cpu.VALUE/100, 2) as CPU_USAGE_SECONDS,
			ss_pga.VALUE as PGA_MEMORY_BYTES,
			ss_logical.VALUE as LOGICAL_READS
		FROM V$SESSION s
		LEFT JOIN V$SESSTAT ss_cpu ON s.SID = ss_cpu.SID 
			AND ss_cpu.STATISTIC# = (SELECT STATISTIC# FROM V$STATNAME WHERE NAME = 'CPU used by this session')
		LEFT JOIN V$SESSTAT ss_pga ON s.SID = ss_pga.SID 
			AND ss_pga.STATISTIC# = (SELECT STATISTIC# FROM V$STATNAME WHERE NAME = 'session pga memory')
		LEFT JOIN V$SESSTAT ss_logical ON s.SID = ss_logical.SID 
			AND ss_logical.STATISTIC# = (SELECT STATISTIC# FROM V$STATNAME WHERE NAME = 'session logical reads')
		WHERE s.TYPE = 'USER'
		ORDER BY ss_cpu.VALUE DESC NULLS LAST
		FETCH FIRST 500 ROWS ONLY`

	// Wait Events & Locks - Current Wait Events
	CurrentWaitEventsSQL = `
		SELECT 
			SID,
			USERNAME,
			EVENT,
			WAIT_TIME,
			STATE,
			SECONDS_IN_WAIT,
			WAIT_CLASS
		FROM V$SESSION 
		WHERE STATUS = 'ACTIVE' 
		AND WAIT_CLASS != 'Idle'
		AND EVENT IS NOT NULL
		ORDER BY SECONDS_IN_WAIT DESC
		FETCH FIRST 200 ROWS ONLY`

	// Blocking Sessions
	BlockingSessionsSQL = `
		SELECT 
			SID,
			SERIAL#,
			BLOCKING_SESSION,
			EVENT,
			USERNAME,
			PROGRAM,
			SECONDS_IN_WAIT
		FROM V$SESSION 
		WHERE BLOCKING_SESSION IS NOT NULL
		ORDER BY SECONDS_IN_WAIT DESC
		FETCH FIRST 100 ROWS ONLY`

	// Wait Event Summary (Top 20 by time waited)
	WaitEventSummarySQL = `
		SELECT 
			EVENT,
			TOTAL_WAITS,
			TIME_WAITED_MICRO,
			CASE 
				WHEN TOTAL_WAITS > 0 THEN TIME_WAITED_MICRO / TOTAL_WAITS 
				ELSE 0 
			END AS AVERAGE_WAIT_MICRO,
			WAIT_CLASS
		FROM V$SYSTEM_EVENT 
		WHERE WAIT_CLASS != 'Idle'
		ORDER BY TIME_WAITED_MICRO DESC 
		FETCH FIRST 20 ROWS ONLY`

	// Connection Pool Metrics
	ConnectionPoolMetricsSQL = `
		SELECT 
			'shared_servers' as METRIC_NAME,
			COUNT(*) as VALUE
		FROM V$SHARED_SERVER
		UNION ALL
		SELECT 
			'dispatchers' as METRIC_NAME,
			COUNT(*) as VALUE
		FROM V$DISPATCHER
		UNION ALL
		SELECT 
			'circuits' as METRIC_NAME,
			COUNT(*) as VALUE
		FROM V$CIRCUIT
		WHERE STATUS = 'NORMAL'`

	// Session Limits
	SessionLimitsSQL = `
		SELECT 
			RESOURCE_NAME,
			CURRENT_UTILIZATION,
			MAX_UTILIZATION,
			INITIAL_ALLOCATION,
			LIMIT_VALUE
		FROM V$RESOURCE_LIMIT 
		WHERE RESOURCE_NAME IN ('sessions', 'processes', 'enqueue_locks', 'enqueue_resources')`

	// Connection Quality Metrics
	ConnectionQualitySQL = `
		SELECT 
			NAME,
			VALUE
		FROM V$SYSSTAT 
		WHERE NAME IN (
			'user commits',
			'user rollbacks', 
			'parse count (total)',
			'parse count (hard)',
			'execute count',
			'SQL*Net roundtrips to/from client',
			'bytes sent via SQL*Net to client',
			'bytes received via SQL*Net from client'
		)`
)
