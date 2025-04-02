// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

// Direct access to queries is not recommended: The receiver allows filtering based on
// instance name, which means the query will change based on configuration.
// Please use getSQLServerDatabaseIOQuery
const sqlServerDatabaseIOQuery = `
SET DEADLOCK_PRIORITY -10;
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard,Enterprise,Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard,Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

DECLARE
	 @SqlStatement AS nvarchar(max)
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int) * 100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)
	,@Columns AS nvarchar(max) = ''
	,@Tables AS nvarchar(max) = ''
IF @MajorMinorVersion > 1100 BEGIN
	SET @Columns += N'
	,vfs.[io_stall_queued_read_ms] AS [rg_read_stall_ms]
	,vfs.[io_stall_queued_write_ms] AS [rg_write_stall_ms]'
END

SET @SqlStatement = N'
SELECT
	''sqlserver_database_io'' AS [measurement]
	,REPLACE(@@SERVERNAME,''\'','':'') AS [sql_instance]
	,HOST_NAME() AS [computer_name]
	,DB_NAME(vfs.[database_id]) AS [database_name]
	,COALESCE(mf.[physical_name],''RBPEX'') AS [physical_filename]	--RPBEX = Resilient Buffer Pool Extension
	,COALESCE(mf.[name],''RBPEX'') AS [logical_filename]	--RPBEX = Resilient Buffer Pool Extension
	,mf.[type_desc] AS [file_type]
	,vfs.[io_stall_read_ms] AS [read_latency_ms]
	,vfs.[num_of_reads] AS [reads]
	,vfs.[num_of_bytes_read] AS [read_bytes]
	,vfs.[io_stall_write_ms] AS [write_latency_ms]
	,vfs.[num_of_writes] AS [writes]
	,vfs.[num_of_bytes_written] AS [write_bytes]'
	+ @Columns + N'
FROM sys.dm_io_virtual_file_stats(NULL, NULL) AS vfs
INNER JOIN sys.master_files AS mf WITH (NOLOCK)
	ON vfs.[database_id] = mf.[database_id] AND vfs.[file_id] = mf.[file_id]
%s'
+ @Tables;

EXEC sp_executesql @SqlStatement
`

func getSQLServerDatabaseIOQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("WHERE @@SERVERNAME = ''%s''", instanceName)
		return fmt.Sprintf(sqlServerDatabaseIOQuery, whereClause)
	}

	return fmt.Sprintf(sqlServerDatabaseIOQuery, "")
}

const sqlServerPerformanceCountersQuery string = `
SET DEADLOCK_PRIORITY -10;
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard,Enterprise,Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

DECLARE
	 @SqlStatement AS nvarchar(max)
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int)*100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)

DECLARE @PCounters TABLE
(
	 [object_name] nvarchar(128)
	,[counter_name] nvarchar(128)
	,[instance_name] nvarchar(128)
	,[cntr_value] bigint
	,[cntr_type] int
	PRIMARY KEY([object_name], [counter_name], [instance_name])
);

WITH PerfCounters AS (
SELECT DISTINCT
	 RTRIM(spi.[object_name]) [object_name]
	,RTRIM(spi.[counter_name]) [counter_name]
	,RTRIM(spi.[instance_name]) AS [instance_name]
	,CAST(spi.[cntr_value] AS bigint) AS [cntr_value]
	,spi.[cntr_type]
	FROM sys.dm_os_performance_counters AS spi
	WHERE
		counter_name IN (
			 'SQL Compilations/sec'
			,'SQL Re-Compilations/sec'
			,'User Connections'
			,'Batch Requests/sec'
			,'Logouts/sec'
			,'Logins/sec'
			,'Processes blocked'
			,'Latch Waits/sec'
			,'Average Latch Wait Time (ms)'
			,'Full Scans/sec'
			,'Index Searches/sec'
			,'Page Splits/sec'
			,'Page lookups/sec'
			,'Page reads/sec'
			,'Page writes/sec'
			,'Readahead pages/sec'
			,'Lazy writes/sec'
			,'Checkpoint pages/sec'
			,'Table Lock Escalations/sec'
			,'Page life expectancy'
			,'Log File(s) Size (KB)'
			,'Log File(s) Used Size (KB)'
			,'Data File(s) Size (KB)'
			,'Transactions/sec'
			,'Write Transactions/sec'
			,'Active Transactions'
			,'Log Growths'
			,'Active Temp Tables'
			,'Logical Connections'
			,'Temp Tables Creation Rate'
			,'Temp Tables For Destruction'
			,'Free Space in tempdb (KB)'
			,'Version Store Size (KB)'
			,'Memory Grants Pending'
			,'Memory Grants Outstanding'
			,'Free list stalls/sec'
			,'Buffer cache hit ratio'
			,'Buffer cache hit ratio base'
			,'Database Pages'
			,'Backup/Restore Throughput/sec'
			,'Total Server Memory (KB)'
			,'Target Server Memory (KB)'
			,'Log Flushes/sec'
			,'Log Flush Wait Time'
			,'Memory broker clerk size'
			,'Log Bytes Flushed/sec'
			,'Bytes Sent to Replica/sec'
			,'Log Send Queue'
			,'Bytes Sent to Transport/sec'
			,'Sends to Replica/sec'
			,'Bytes Sent to Transport/sec'
			,'Sends to Transport/sec'
			,'Bytes Received from Replica/sec'
			,'Receives from Replica/sec'
			,'Flow Control Time (ms/sec)'
			,'Flow Control/sec'
			,'Resent Messages/sec'
			,'Redone Bytes/sec'
			,'XTP Memory Used (KB)'
			,'Transaction Delay'
			,'Log Bytes Received/sec'
			,'Log Apply Pending Queue'
			,'Redone Bytes/sec'
			,'Recovery Queue'
			,'Log Apply Ready Queue'
			,'CPU usage %'
			,'CPU usage % base'
			,'Queued requests'
			,'Requests completed/sec'
			,'Blocked tasks'
			,'Active memory grant amount (KB)'
			,'Disk Read Bytes/sec'
			,'Disk Read IO Throttled/sec'
			,'Disk Read IO/sec'
			,'Disk Write Bytes/sec'
			,'Disk Write IO Throttled/sec'
			,'Disk Write IO/sec'
			,'Used memory (KB)'
			,'Forwarded Records/sec'
			,'Background Writer pages/sec'
			,'Percent Log Used'
			,'Log Send Queue KB'
			,'Redo Queue KB'
			,'Mirrored Write Transactions/sec'
			,'Group Commit Time'
			,'Group Commits/Sec'
			,'Workfiles Created/sec'
			,'Worktables Created/sec'
			,'Distributed Query'
			,'DTC calls'
			,'Query Store CPU usage'
			,'Query Store physical reads'
			,'Query Store logical reads'
			,'Query Store logical writes'
			,'Execution Errors'
		) OR (
			spi.[object_name] LIKE '%User Settable%'
			OR spi.[object_name] LIKE '%SQL Errors%'
			OR spi.[object_name] LIKE '%Batch Resp Statistics%'
		) OR (
			spi.[instance_name] IN ('_Total')
			AND spi.[counter_name] IN (
				 'Lock Timeouts/sec'
				,'Lock Timeouts (timeout > 0)/sec'
				,'Number of Deadlocks/sec'
				,'Lock Waits/sec'
				,'Latch Waits/sec'
			)
		)
)

INSERT INTO @PCounters SELECT * FROM PerfCounters;

SELECT
	 'sqlserver_performance' AS [measurement]
	,REPLACE(@@SERVERNAME,'\',':') AS [sql_instance]
	,HOST_NAME() AS [computer_name]
	,pc.[object_name] AS [object]
	,pc.[counter_name] AS [counter]
	,CASE pc.[instance_name] WHEN '_Total' THEN 'Total' ELSE ISNULL(pc.[instance_name],'') END AS [instance]
	,CAST(CASE WHEN pc.[cntr_type] = 537003264 AND pc1.[cntr_value] > 0 THEN (pc.[cntr_value] * 1.0) / (pc1.[cntr_value] * 1.0) * 100 ELSE pc.[cntr_value] END AS float(10)) AS [value]
	,CAST(pc.[cntr_type] AS varchar(25)) AS [counter_type]
FROM @PCounters AS pc
LEFT OUTER JOIN @PCounters AS pc1
	ON (
		pc.[counter_name] = REPLACE(pc1.[counter_name],' base','')
		OR pc.[counter_name] = REPLACE(pc1.[counter_name],' base',' (ms)')
	)
	AND pc.[object_name] = pc1.[object_name]
	AND pc.[instance_name] = pc1.[instance_name]
	AND pc1.[counter_name] LIKE '%base'
WHERE
	pc.[counter_name] NOT LIKE '% base'
{filter_instance_name}
OPTION(RECOMPILE)
`

func getSQLServerPerformanceCounterQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("\tAND @@SERVERNAME = '%s'", instanceName)
		r := strings.NewReplacer("{filter_instance_name}", whereClause)
		return r.Replace(sqlServerPerformanceCountersQuery)
	}

	r := strings.NewReplacer("{filter_instance_name}", "")
	return r.Replace(sqlServerPerformanceCountersQuery)
}

const sqlServerProperties = `
SET DEADLOCK_PRIORITY -10;
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard, Enterprise, Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

DECLARE
	 @SqlStatement AS nvarchar(max) = ''
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int)*100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)
	,@Columns AS nvarchar(MAX) = ''

IF CAST(SERVERPROPERTY('ProductVersion') AS varchar(50)) >= '10.50.2500.0'
	SET @Columns = N'
	,CASE [virtual_machine_type_desc]
		WHEN ''NONE'' THEN ''PHYSICAL Machine''
		ELSE [virtual_machine_type_desc]
	END AS [hardware_type]'

SET @SqlStatement = '
DECLARE @ForceEncryption INT
DECLARE @DynamicportNo NVARCHAR(50);
DECLARE @StaticportNo NVARCHAR(50);

EXEC [xp_instance_regread]
	 @rootkey = ''HKEY_LOCAL_MACHINE''
	,@key = ''SOFTWARE\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib''
	,@value_name = ''ForceEncryption''
	,@value = @ForceEncryption OUTPUT;

EXEC [xp_instance_regread]
	 @rootkey = ''HKEY_LOCAL_MACHINE''
	,@key = ''Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll''
	,@value_name = ''TcpDynamicPorts''
	,@value = @DynamicportNo OUTPUT

EXEC [xp_instance_regread]
	  @rootkey = ''HKEY_LOCAL_MACHINE''
     ,@key = ''Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll''
     ,@value_name = ''TcpPort''
     ,@value = @StaticportNo OUTPUT

SELECT
	 ''sqlserver_server_properties'' AS [measurement]
	,REPLACE(@@SERVERNAME,''\'','':'') AS [sql_instance]
	,HOST_NAME() AS [computer_name]
	,@@SERVICENAME AS [service_name]
	,si.[cpu_count]
	,(SELECT [total_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [server_memory]
	,(SELECT [available_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [available_server_memory]
	,SERVERPROPERTY(''Edition'') AS [sku]
	,CAST(SERVERPROPERTY(''EngineEdition'') AS int) AS [engine_edition]
	,DATEDIFF(MINUTE,si.[sqlserver_start_time],GETDATE()) AS [uptime]
	,SERVERPROPERTY(''ProductVersion'') AS [sql_version]
	,SERVERPROPERTY(''IsClustered'') AS [instance_type]
	,SERVERPROPERTY(''IsHadrEnabled'') AS [is_hadr_enabled]
	,LEFT(@@VERSION,CHARINDEX('' - '',@@VERSION)) AS [sql_version_desc]
	,@ForceEncryption AS [ForceEncryption]
	,COALESCE(@DynamicportNo,@StaticportNo) AS [Port]
	,IIF(@DynamicportNo IS NULL, ''Static'', ''Dynamic'') AS [PortType]
	,dbs.[db_online]
	,dbs.[db_restoring]
	,dbs.[db_recovering]
	,dbs.[db_recoveryPending]
	,dbs.[db_suspect]
	,dbs.[db_offline]'
	+ @Columns + N'
	FROM sys.[dm_os_sys_info] AS si
	CROSS APPLY (
		SELECT
			 SUM(CASE WHEN [state] = 0 THEN 1 ELSE 0 END) AS [db_online]
			,SUM(CASE WHEN [state] = 1 THEN 1 ELSE 0 END) AS [db_restoring]
			,SUM(CASE WHEN [state] = 2 THEN 1 ELSE 0 END) AS [db_recovering]
			,SUM(CASE WHEN [state] = 3 THEN 1 ELSE 0 END) AS [db_recoveryPending]
			,SUM(CASE WHEN [state] = 4 THEN 1 ELSE 0 END) AS [db_suspect]
			,SUM(CASE WHEN [state] IN (6,10) THEN 1 ELSE 0 END) AS [db_offline]
		FROM sys.databases
	) AS dbs
%s'

EXEC sp_executesql @SqlStatement
`

func getSQLServerPropertiesQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("WHERE @@SERVERNAME = ''%s''", instanceName)
		return fmt.Sprintf(sqlServerProperties, whereClause)
	}

	return fmt.Sprintf(sqlServerProperties, "")
}

//go:embed templates/dbQueryAndTextQuery.tmpl
var sqlServerQueryTextAndPlanQueryTemplate string

func getSQLServerQueryTextAndPlanQuery(instanceName string, maxQuerySampleCount uint, lookbackTime uint) (string, error) {
	var instanceNameClause string

	if instanceName != "" {
		instanceNameClause = fmt.Sprintf("AND @@SERVERNAME = '%s'", instanceName)
	} else {
		instanceNameClause = ""
	}

	tmpl := template.Must(template.New("dbQueryAndTextQuery").Option("missingkey=error").Parse(sqlServerQueryTextAndPlanQueryTemplate))
	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, map[string]any{
		"topNValue":          maxQuerySampleCount,
		"lookbackTime":       lookbackTime,
		"instanceNameClause": instanceNameClause,
	}); err != nil {
		return "", fmt.Errorf("failed executing template: %w", err)
	}

	return buf.String(), nil
}

//go:embed templates/sqlServerQuerySampleWithTop.tmpl
var sqlServerQuerySamplesWithTop string

// This one is used when the template execution fails as a fallback.
// Even though in theory the template function should never fail, we
// have it just in case.
//
//go:embed templates/sqlServerQuerySampleWithoutTop.tmpl
var sqlServerQuerySamplesWithoutTop string

func getSQLServerQuerySamplesQuery(maxRowsPerQuery uint64) string {
	buf := bytes.Buffer{}

	tmpl := template.Must(template.New("sqlServerQuerySample").Option("missingkey=error").Parse(sqlServerQuerySamplesWithTop))
	if err := tmpl.Execute(&buf, map[string]any{
		"Limit": maxRowsPerQuery,
	}); err != nil {
		return sqlServerQuerySamplesWithoutTop
	}
	return buf.String()
}
