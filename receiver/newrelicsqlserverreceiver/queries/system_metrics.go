// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for system-level metrics.
// This file contains SQL queries for collecting comprehensive host and SQL Server system information
// that is automatically added as resource attributes to all metrics.
package queries

// SystemInformationQuery returns comprehensive system and host information for SQL Server instance
// This query collects essential host/system context that should be included as resource attributes
// with all metrics sent by the scraper
const SystemInformationQuery = `
SET DEADLOCK_PRIORITY -10;

-- Handle different SQL Server editions
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4,5,8) BEGIN /*NOT IN Standard,Enterprise,Express,Azure SQL Database, Azure SQL Managed Instance*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise, Express, Azure SQL Database or Azure SQL Managed Instance. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

DECLARE
	 @ForceEncryption AS int = 0
	,@StaticportNo AS nvarchar(20) = NULL
	,@DynamicportNo AS nvarchar(20) = NULL

-- Get port configuration (only for non-Azure SQL Database)
IF SERVERPROPERTY('EngineEdition') != 5 BEGIN
	-- Try to get dynamic port
	EXEC xp_instance_regread
		 @rootkey = 'HKEY_LOCAL_MACHINE'
		,@key = 'Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll'
		,@value_name = 'TcpDynamicPorts'
		,@value = @DynamicportNo OUTPUT

	-- Try to get static port if dynamic port is null
	IF @DynamicportNo IS NULL OR @DynamicportNo = ''
	EXEC xp_instance_regread
		 @rootkey = 'HKEY_LOCAL_MACHINE'
		,@key = 'Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll'
		,@value_name = 'TcpPort'
		,@value = @StaticportNo OUTPUT

	-- Get encryption setting
	EXEC xp_instance_regread
		 @rootkey = 'HKEY_LOCAL_MACHINE'
		,@key = 'Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib'
		,@value_name = 'ForceEncryption'
		,@value = @ForceEncryption OUTPUT
END

SELECT
	 REPLACE(@@SERVERNAME,'\',':') AS [sql_instance]
	,HOST_NAME() AS [computer_name]
	,@@SERVICENAME AS [service_name]
	,si.[cpu_count]
	,(SELECT [total_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [server_memory]
	,(SELECT [available_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [available_server_memory]
	,SERVERPROPERTY('Edition') AS [sku]
	,CAST(SERVERPROPERTY('EngineEdition') AS int) AS [engine_edition]
	,DATEDIFF(MINUTE,si.[sqlserver_start_time],GETDATE()) AS [uptime]
	,SERVERPROPERTY('ProductVersion') AS [sql_version]
	,SERVERPROPERTY('IsClustered') AS [instance_type]
	,SERVERPROPERTY('IsHadrEnabled') AS [is_hadr_enabled]
	,LEFT(@@VERSION,CHARINDEX(' - ',@@VERSION)) AS [sql_version_desc]
	,@ForceEncryption AS [ForceEncryption]
	,COALESCE(@DynamicportNo,@StaticportNo) AS [Port]
	,IIF(@DynamicportNo IS NULL, 'Static', 'Dynamic') AS [PortType]
	,(si.[ms_ticks]/1000) AS [computer_uptime]
	FROM sys.[dm_os_sys_info] AS si`
