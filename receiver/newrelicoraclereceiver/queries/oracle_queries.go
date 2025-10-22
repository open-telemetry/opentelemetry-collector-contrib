// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

// Oracle SQL query for session count metric
const (
	SessionCountSQL = "SELECT COUNT(*) as SESSION_COUNT FROM v$session WHERE type = 'USER'"

	// RAC-specific queries for cluster monitoring

	// ASM Disk Group monitoring query
	ASMDiskGroupSQL = `
		SELECT
			NAME,
			TOTAL_MB,
			FREE_MB,
			OFFLINE_DISKS
		FROM GV$ASM_DISKGROUP`

	// Cluster wait events query
	ClusterWaitEventsSQL = `
		SELECT
			INST_ID,
			EVENT,
			TOTAL_WAITS,
			TIME_WAITED_MICRO
		FROM GV$SYSTEM_EVENT
		WHERE WAIT_CLASS = 'Cluster'`

	// RAC instance status query
	RACInstanceStatusSQL = `
		SELECT
			INST_ID,
			INSTANCE_NAME,
			HOST_NAME,
			STATUS,
			STARTUP_TIME,
			DATABASE_STATUS,
			ACTIVE_STATE,
			LOGINS,
			ARCHIVER,
			VERSION
		FROM GV$INSTANCE`

	// Active services query for failover tracking
	RACActiveServicesSQL = `
		SELECT
			NAME AS SERVICE_NAME,
			INST_ID,
			FAILOVER_METHOD,
			FAILOVER_TYPE,
			GOAL,
			NETWORK_NAME,
			CREATION_DATE,
			FAILOVER_RETRIES,
			FAILOVER_DELAY,
			CLB_GOAL
		FROM GV$ACTIVE_SERVICES`

	// RAC detection query - checks if Oracle is running in RAC mode
	RACDetectionSQL = `
		SELECT 
			VALUE 
		FROM V$PARAMETER 
		WHERE NAME = 'cluster_database'`
)

// Oracle SQL query for system metrics from gv$sysmetric
const (
	SystemSysMetricsSQL = "SELECT INST_ID, METRIC_NAME, VALUE FROM gv$sysmetric"
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

	// Alternative for PDB context - uses current container's data files
	PDBDatafilesOfflineCurrentContainerSQL = `
		SELECT
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_DATAFILES_OFFLINE,
			TABLESPACE_NAME
		FROM dba_data_files
		GROUP BY TABLESPACE_NAME`

	PDBNonWriteTablespaceSQL = `
		SELECT 
			TABLESPACE_NAME, 
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_NON_WRITE_MODE
		FROM cdb_data_files a, cdb_pdbs b
		WHERE a.con_id = b.con_id
		GROUP BY TABLESPACE_NAME`

	// Alternative for PDB context - uses current container's data files
	PDBNonWriteCurrentContainerSQL = `
		SELECT 
			TABLESPACE_NAME, 
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_NON_WRITE_MODE
		FROM dba_data_files
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

	// Alternative for PDB context - uses current container's users
	LockedAccountsCurrentContainerSQL = `
		SELECT
			INST_ID, LOCKED_ACCOUNTS
		FROM
		(	SELECT count(1) AS LOCKED_ACCOUNTS
			FROM
				dba_users
			WHERE account_status != 'OPEN'
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

	DBIDInstanceSQL = `
		SELECT
			t1.INST_ID,
			t2.DBID
		FROM (SELECT INST_ID FROM gv$instance) t1,
		(SELECT DBID FROM v$database) t2`

	LongRunningQueriesSQL = `
		SELECT inst_id, sum(num) AS total FROM ((
			SELECT i.inst_id, 1 AS num
			FROM gv$session s, gv$instance i
			WHERE i.inst_id=s.inst_id
			AND s.status='ACTIVE'
			AND s.type <>'BACKGROUND'
			AND s.last_call_et > 60
			GROUP BY i.inst_id
		) UNION (
			SELECT i.inst_id, 0 AS num
			FROM gv$session s, gv$instance i
			WHERE i.inst_id=s.inst_id
		))
		GROUP BY inst_id`

	SGAUGATotalMemorySQL = `
		SELECT SUM(value) AS sum, inst.inst_id
		FROM GV$sesstat, GV$statname, GV$INSTANCE inst
		WHERE name = 'session uga memory max'
		AND GV$sesstat.statistic#=GV$statname.statistic#
		AND GV$sesstat.inst_id=inst.inst_id
		AND GV$statname.inst_id=inst.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheShareableStatementSQL = `
		SELECT SUM(sqlarea.sharable_mem) AS sum, inst.inst_id
		FROM GV$sqlarea sqlarea, GV$INSTANCE inst
		WHERE sqlarea.executions > 5
		AND inst.inst_id=sqlarea.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheShareableUserSQL = `
		SELECT SUM(250 * sqlarea.users_opening) AS sum, inst.inst_id
		FROM GV$sqlarea sqlarea, GV$INSTANCE inst
		WHERE inst.inst_id=sqlarea.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheReloadRatioSQL = `
		SELECT (sum(libcache.reloads)/sum(libcache.pins)) AS ratio, inst.inst_id
		FROM GV$librarycache libcache, GV$INSTANCE inst
		WHERE inst.inst_id=libcache.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheHitRatioSQL = `
		SELECT libcache.gethitratio as ratio, inst.inst_id
		FROM GV$librarycache libcache, GV$INSTANCE inst
		WHERE namespace='SQL AREA'
		AND inst.inst_id=libcache.inst_id`

	SGASharedPoolDictCacheMissRatioSQL = `
		SELECT (SUM(rcache.getmisses)/SUM(rcache.gets)) as ratio, inst.inst_id
		FROM GV$rowcache rcache, GV$INSTANCE inst
		WHERE inst.inst_id=rcache.inst_id
		GROUP BY inst.inst_id`

	SGALogBufferSpaceWaitsSQL = `
		SELECT count(wait.inst_id) as count, inst.inst_id
		FROM GV$SESSION_WAIT wait, GV$INSTANCE inst
		WHERE wait.event like 'log buffer space%'
		AND inst.inst_id=wait.inst_id
		GROUP BY inst.inst_id`

	SGALogAllocRetriesSQL = `
		SELECT (rbar.value/re.value) as ratio, inst.inst_id
		FROM GV$SYSSTAT rbar, GV$SYSSTAT re, GV$INSTANCE inst
		WHERE rbar.name like 'redo buffer allocation retries'
		AND re.name like 'redo entries'
		AND re.inst_id=inst.inst_id AND rbar.inst_id=inst.inst_id`

	SGAHitRatioSQL = `
		SELECT inst.inst_id,(1 - (phy.value - lob.value - dir.value)/ses.value) as ratio
		FROM GV$SYSSTAT ses, GV$SYSSTAT lob, GV$SYSSTAT dir, GV$SYSSTAT phy, GV$INSTANCE inst
		WHERE ses.name='session logical reads'
		AND dir.name='physical reads direct'
		AND lob.name='physical reads direct (lob)'
		AND phy.name='physical reads'
		AND ses.inst_id=inst.inst_id
		AND lob.inst_id=inst.inst_id
		AND dir.inst_id=inst.inst_id
		AND phy.inst_id=inst.inst_id`

	SysstatSQL = `
		SELECT inst.inst_id, sysstat.name, sysstat.value
		FROM GV$SYSSTAT sysstat, GV$INSTANCE inst
		WHERE sysstat.inst_id=inst.inst_id 
		AND sysstat.name IN ('redo buffer allocation retries', 'redo entries', 'sorts (memory)', 'sorts (disk)')`

	SGASQL = `
		SELECT inst.inst_id, sga.name, sga.value
		FROM GV$SGA sga, GV$INSTANCE inst
		WHERE sga.inst_id=inst.inst_id 
		AND sga.name IN ('Fixed Size', 'Redo Buffers')`

	RollbackSegmentsSQL = `
		SELECT
			SUM(stat.gets) AS gets,
			sum(stat.waits) AS waits,
			sum(stat.waits)/sum(stat.gets) AS ratio,
			inst.inst_id
		FROM GV$ROLLSTAT stat, GV$INSTANCE inst
		WHERE stat.inst_id=inst.inst_id
		GROUP BY inst.inst_id`

	RedoLogWaitsSQL = `
		SELECT
			sysevent.total_waits,
			inst.inst_id,
			sysevent.event
		FROM
			GV$SYSTEM_EVENT sysevent,
			GV$INSTANCE inst
		WHERE sysevent.inst_id=inst.inst_id
		AND (sysevent.event LIKE '%log file parallel write%'
		OR sysevent.event LIKE '%log file switch completion%'
		OR sysevent.event LIKE '%log file switch (check%'
		OR sysevent.event LIKE '%log file switch (arch%'
		OR sysevent.event LIKE '%buffer busy waits%'
		OR sysevent.event LIKE '%freeBufferWaits%'
		OR sysevent.event LIKE '%free buffer inspected%')`

	PDBSysMetricsSQL = `
		SELECT
			INST_ID,
			METRIC_NAME,
			VALUE
		FROM gv$con_sysmetric`

	// Container Database (CDB) Level Metrics
	ContainerStatusSQL = `
		SELECT 
			CON_ID,
			NAME as CONTAINER_NAME,
			OPEN_MODE,
			RESTRICTED,
			OPEN_TIME
		FROM GV$CONTAINERS
		WHERE ROWNUM <= 1000` // Limit for performance

	PDBStatusSQL = `
		SELECT 
			CON_ID,
			NAME as PDB_NAME,
			CREATE_SCN,
			OPEN_MODE,
			RESTRICTED,
			OPEN_TIME,
			TOTAL_SIZE
		FROM GV$PDBS
		WHERE ROWNUM <= 1000` // Limit for performance

	CDBTablespaceUsageSQL = `
		SELECT 
			con_id,
			tablespace_name,
			used_space * block_size as used_bytes,
			tablespace_size * block_size as total_bytes,
			used_percent
		FROM CDB_TABLESPACE_USAGE_METRICS
		WHERE ROWNUM <= 5000` // Reasonable limit for tablespaces

	CDBDataFilesSQL = `
		SELECT 
			con_id,
			file_name,
			tablespace_name,
			bytes,
			status,
			autoextensible,
			maxbytes,
			user_bytes
		FROM CDB_DATA_FILES
		WHERE ROWNUM <= 10000` // Limit for performance

	CDBServicesSQL = `
		SELECT 
			con_id,
			name as service_name,
			network_name,
			creation_date,
			pdb,
			enabled
		FROM CDB_SERVICES
		WHERE ROWNUM <= 1000` // Reasonable service limit

	ContainerSysMetricsSQL = `
		SELECT 
			con_id,
			metric_name,
			value,
			metric_unit,
			group_id
		FROM GV$CON_SYSMETRIC 
		WHERE group_id = 2
		AND ROWNUM <= 5000` // Limit for performance

	// Alternative query for when connected to CDB$ROOT
	CDBSysMetricsSQL = `
		SELECT 
			con_id,
			metric_name,
			value,
			metric_unit,
			group_id
		FROM GV$SYSMETRIC 
		WHERE group_id = 2
		AND ROWNUM <= 5000` // Limit for performance

	// Environment detection queries
	CheckCDBFeatureSQL = `
		SELECT 
			CASE WHEN CDB = 'YES' THEN 1 ELSE 0 END as IS_CDB
		FROM V$DATABASE`

	CheckPDBCapabilitySQL = `
		SELECT COUNT(*) as PDB_COUNT
		FROM ALL_TABLES 
		WHERE TABLE_NAME = 'CDB_PDBS' 
		AND OWNER = 'SYS'`

	// ASM detection query - checks if ASM instance is available
	ASMDetectionSQL = `
		SELECT COUNT(*) as ASM_COUNT
		FROM ALL_TABLES 
		WHERE TABLE_NAME = 'GV$ASM_DISKGROUP' 
		AND OWNER = 'SYS'`

	// Check current container context
	CheckCurrentContainerSQL = `
		SELECT 
			SYS_CONTEXT('USERENV', 'CON_NAME') as CONTAINER_NAME,
			SYS_CONTEXT('USERENV', 'CON_ID') as CONTAINER_ID
		FROM DUAL`

	// Database Version and Hosting Information Queries
	// Following Oracle best practices for system information collection

	// Enhanced database info query - combines version with hosting environment hints
	// Includes hostname and platform information to assist cloud provider detection
	// These values are cached to avoid repeated queries and used alongside environment detection
	OptimizedDatabaseInfoSQL = `
		SELECT 
			i.INST_ID,
			i.VERSION as VERSION_FULL,
			i.HOST_NAME,
			d.NAME as DATABASE_NAME,
			d.PLATFORM_NAME
		FROM 
			GV$INSTANCE i,
			V$DATABASE d
		WHERE 
			i.INST_ID = 1
			AND ROWNUM = 1`
)
