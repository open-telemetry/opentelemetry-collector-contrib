// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for user connection metrics.
// This file contains all SQL queries for collecting user connection status information.
//
// User Connection Metrics Queries:
//
// 1. Connection Status Distribution:
//   - Groups user connections by their current status
//   - Provides insight into connection behavior and potential issues
//   - Source: sys.dm_exec_sessions with user process filtering
//
// 2. Connection Summary Statistics:
//   - Aggregated counts for different connection states
//   - Pre-calculated metrics for common monitoring scenarios
//   - Optimized for dashboard and alerting use cases
//
// 3. Connection Utilization Analysis:
//   - Calculates ratios and efficiency metrics
//   - Helps identify connection pool optimization opportunities
//   - Provides percentage-based metrics for trend analysis
//
// Query Compatibility:
// - Standard SQL Server: Full access to all session information
// - Azure SQL Database: Complete session visibility within database scope
// - Azure SQL Managed Instance: Full functionality with all connection states
//
// Performance Considerations:
// - Uses sys.dm_exec_sessions which is lightweight and fast
// - Filters to user processes only (is_user_process = 1)
// - Minimal resource impact suitable for frequent collection
// - No locks or blocking operations
package queries

// UserConnectionStatusQuery returns the SQL query for user connection status distribution
// This query groups user connections by their current status to identify patterns and issues
//
// The query returns:
// - status: Current status of the user session (running, sleeping, suspended, etc.)
// - session_count: Number of user sessions in each status
//
// This is particularly useful for:
// - Identifying connection pooling issues (high sleeping count)
// - Detecting resource contention (high suspended count)
// - Monitoring active workload (running/runnable counts)
const UserConnectionStatusQuery = `SELECT 
    status,
    COUNT(session_id) AS session_count
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1
GROUP BY status`

// UserConnectionStatusQueryAzureSQL returns the same query for Azure SQL Database
// Azure SQL Database supports full session monitoring within the database scope
const UserConnectionStatusQueryAzureSQL = `SELECT 
    status,
    COUNT(session_id) AS session_count
FROM sys.dm_exec_sessions
WHERE is_user_process = 1
GROUP BY status`

// UserConnectionStatusQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance has full session monitoring capabilities
const UserConnectionStatusQueryAzureMI = `SELECT 
    status,
    COUNT(session_id) AS session_count
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1
GROUP BY status`

// UserConnectionSummaryQuery returns aggregated user connection statistics
// This query provides pre-calculated summary metrics for common monitoring scenarios
//
// The query returns:
// - total_user_connections: Total count of user connections
// - sleeping_connections: Count of idle connections
// - running_connections: Count of actively executing connections
// - suspended_connections: Count of connections waiting for resources
// - runnable_connections: Count of connections ready to run
// - dormant_connections: Count of dormant connections
const UserConnectionSummaryQuery = `SELECT 
    COUNT(*) AS total_user_connections,
    SUM(CASE WHEN status = 'sleeping' THEN 1 ELSE 0 END) AS sleeping_connections,
    SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running_connections,
    SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS suspended_connections,
    SUM(CASE WHEN status = 'runnable' THEN 1 ELSE 0 END) AS runnable_connections,
    SUM(CASE WHEN status = 'dormant' THEN 1 ELSE 0 END) AS dormant_connections
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1`

// UserConnectionSummaryQueryAzureSQL returns the summary query for Azure SQL Database
const UserConnectionSummaryQueryAzureSQL = `SELECT 
    COUNT(*) AS total_user_connections,
    SUM(CASE WHEN status = 'sleeping' THEN 1 ELSE 0 END) AS sleeping_connections,
    SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running_connections,
    SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS suspended_connections,
    SUM(CASE WHEN status = 'runnable' THEN 1 ELSE 0 END) AS runnable_connections,
    SUM(CASE WHEN status = 'dormant' THEN 1 ELSE 0 END) AS dormant_connections
FROM sys.dm_exec_sessions
WHERE is_user_process = 1`

// UserConnectionSummaryQueryAzureMI returns the summary query for Azure SQL Managed Instance
const UserConnectionSummaryQueryAzureMI = `SELECT 
    COUNT(*) AS total_user_connections,
    SUM(CASE WHEN status = 'sleeping' THEN 1 ELSE 0 END) AS sleeping_connections,
    SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running_connections,
    SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS suspended_connections,
    SUM(CASE WHEN status = 'runnable' THEN 1 ELSE 0 END) AS runnable_connections,
    SUM(CASE WHEN status = 'dormant' THEN 1 ELSE 0 END) AS dormant_connections
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1`

// UserConnectionUtilizationQuery returns connection utilization metrics
// This query calculates percentage-based metrics for connection efficiency analysis
//
// The query returns:
// - active_connection_ratio: Percentage of connections actively working
// - idle_connection_ratio: Percentage of connections that are idle
// - waiting_connection_ratio: Percentage of connections waiting for resources
// - connection_efficiency: Overall connection efficiency metric
const UserConnectionUtilizationQuery = `WITH ConnectionStats AS (
    SELECT 
        COUNT(*) AS total_connections,
        SUM(CASE WHEN status IN ('running', 'runnable') THEN 1 ELSE 0 END) AS active_connections,
        SUM(CASE WHEN status IN ('sleeping', 'dormant') THEN 1 ELSE 0 END) AS idle_connections,
        SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS waiting_connections
    FROM sys.dm_exec_sessions WITH (NOLOCK)
    WHERE is_user_process = 1
)
SELECT 
    CASE WHEN total_connections > 0 
        THEN (active_connections * 100.0) / total_connections 
        ELSE 0 
    END AS active_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (idle_connections * 100.0) / total_connections 
        ELSE 0 
    END AS idle_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (waiting_connections * 100.0) / total_connections 
        ELSE 0 
    END AS waiting_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN 100.0 - ((idle_connections * 100.0) / total_connections)
        ELSE 0 
    END AS connection_efficiency
FROM ConnectionStats`

// UserConnectionUtilizationQueryAzureSQL returns the utilization query for Azure SQL Database
const UserConnectionUtilizationQueryAzureSQL = `WITH ConnectionStats AS (
    SELECT 
        COUNT(*) AS total_connections,
        SUM(CASE WHEN status IN ('running', 'runnable') THEN 1 ELSE 0 END) AS active_connections,
        SUM(CASE WHEN status IN ('sleeping', 'dormant') THEN 1 ELSE 0 END) AS idle_connections,
        SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS waiting_connections
    FROM sys.dm_exec_sessions
    WHERE is_user_process = 1
)
SELECT 
    CASE WHEN total_connections > 0 
        THEN (active_connections * 100.0) / total_connections 
        ELSE 0 
    END AS active_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (idle_connections * 100.0) / total_connections 
        ELSE 0 
    END AS idle_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (waiting_connections * 100.0) / total_connections 
        ELSE 0 
    END AS waiting_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN 100.0 - ((idle_connections * 100.0) / total_connections)
        ELSE 0 
    END AS connection_efficiency
FROM ConnectionStats`

// UserConnectionUtilizationQueryAzureMI returns the utilization query for Azure SQL Managed Instance
const UserConnectionUtilizationQueryAzureMI = `WITH ConnectionStats AS (
    SELECT 
        COUNT(*) AS total_connections,
        SUM(CASE WHEN status IN ('running', 'runnable') THEN 1 ELSE 0 END) AS active_connections,
        SUM(CASE WHEN status IN ('sleeping', 'dormant') THEN 1 ELSE 0 END) AS idle_connections,
        SUM(CASE WHEN status = 'suspended' THEN 1 ELSE 0 END) AS waiting_connections
    FROM sys.dm_exec_sessions WITH (NOLOCK)
    WHERE is_user_process = 1
)
SELECT 
    CASE WHEN total_connections > 0 
        THEN (active_connections * 100.0) / total_connections 
        ELSE 0 
    END AS active_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (idle_connections * 100.0) / total_connections 
        ELSE 0 
    END AS idle_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN (waiting_connections * 100.0) / total_connections 
        ELSE 0 
    END AS waiting_connection_ratio,
    CASE WHEN total_connections > 0 
        THEN 100.0 - ((idle_connections * 100.0) / total_connections)
        ELSE 0 
    END AS connection_efficiency
FROM ConnectionStats`

// UserConnectionByClientQuery returns the SQL query for user connections grouped by client and program
// This query shows connection distribution by source host and application
//
// The query returns:
// - host_name: Name of the client host/machine making the connection
// - program_name: Name of the client application/program
// - connection_count: Number of connections from this host/program combination
//
// This is particularly useful for:
// - Identifying which applications generate the most connections
// - Understanding connection source distribution
// - Detecting potential connection pooling issues by application
// - Monitoring client-side connection patterns
const UserConnectionByClientQuery = `SELECT 
    ISNULL(host_name, 'Unknown') AS host_name,
    ISNULL(program_name, 'Unknown') AS program_name,
    COUNT(session_id) AS connection_count
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1
GROUP BY host_name, program_name
ORDER BY connection_count DESC`

// UserConnectionByClientQueryAzureSQL returns the same query for Azure SQL Database
const UserConnectionByClientQueryAzureSQL = `SELECT 
    ISNULL(host_name, 'Unknown') AS host_name,
    ISNULL(program_name, 'Unknown') AS program_name,
    COUNT(session_id) AS connection_count
FROM sys.dm_exec_sessions
WHERE is_user_process = 1
GROUP BY host_name, program_name
ORDER BY connection_count DESC`

// UserConnectionByClientQueryAzureMI returns the same query for Azure SQL Managed Instance
const UserConnectionByClientQueryAzureMI = `SELECT 
    ISNULL(host_name, 'Unknown') AS host_name,
    ISNULL(program_name, 'Unknown') AS program_name,
    COUNT(session_id) AS connection_count
FROM sys.dm_exec_sessions WITH (NOLOCK)
WHERE is_user_process = 1
GROUP BY host_name, program_name
ORDER BY connection_count DESC`

// UserConnectionClientSummaryQuery returns aggregated statistics about client connections
// This query provides summary metrics for connection source analysis
//
// The query returns:
// - unique_hosts: Count of distinct client hosts
// - unique_programs: Count of distinct programs/applications
// - top_host_connection_count: Highest connection count from a single host
// - top_program_connection_count: Highest connection count from a single program
// - hosts_with_multiple_programs: Count of hosts running multiple programs
// - programs_from_multiple_hosts: Count of programs connecting from multiple hosts
const UserConnectionClientSummaryQuery = `WITH ClientStats AS (
    SELECT 
        host_name,
        program_name,
        COUNT(session_id) AS connection_count
    FROM sys.dm_exec_sessions WITH (NOLOCK)
    WHERE is_user_process = 1
      AND host_name IS NOT NULL
      AND program_name IS NOT NULL
    GROUP BY host_name, program_name
),
HostStats AS (
    SELECT 
        host_name,
        COUNT(DISTINCT program_name) AS programs_per_host,
        SUM(connection_count) AS connections_per_host
    FROM ClientStats
    GROUP BY host_name
),
ProgramStats AS (
    SELECT 
        program_name,
        COUNT(DISTINCT host_name) AS hosts_per_program,
        SUM(connection_count) AS connections_per_program
    FROM ClientStats
    GROUP BY program_name
)
SELECT 
    (SELECT COUNT(DISTINCT host_name) FROM ClientStats) AS unique_hosts,
    (SELECT COUNT(DISTINCT program_name) FROM ClientStats) AS unique_programs,
    (SELECT MAX(connections_per_host) FROM HostStats) AS top_host_connection_count,
    (SELECT MAX(connections_per_program) FROM ProgramStats) AS top_program_connection_count,
    (SELECT COUNT(*) FROM HostStats WHERE programs_per_host > 1) AS hosts_with_multiple_programs,
    (SELECT COUNT(*) FROM ProgramStats WHERE hosts_per_program > 1) AS programs_from_multiple_hosts`

// UserConnectionClientSummaryQueryAzureSQL returns the client summary query for Azure SQL Database
const UserConnectionClientSummaryQueryAzureSQL = `WITH ClientStats AS (
    SELECT 
        host_name,
        program_name,
        COUNT(session_id) AS connection_count
    FROM sys.dm_exec_sessions
    WHERE is_user_process = 1
      AND host_name IS NOT NULL
      AND program_name IS NOT NULL
    GROUP BY host_name, program_name
),
HostStats AS (
    SELECT 
        host_name,
        COUNT(DISTINCT program_name) AS programs_per_host,
        SUM(connection_count) AS connections_per_host
    FROM ClientStats
    GROUP BY host_name
),
ProgramStats AS (
    SELECT 
        program_name,
        COUNT(DISTINCT host_name) AS hosts_per_program,
        SUM(connection_count) AS connections_per_program
    FROM ClientStats
    GROUP BY program_name
)
SELECT 
    (SELECT COUNT(DISTINCT host_name) FROM ClientStats) AS unique_hosts,
    (SELECT COUNT(DISTINCT program_name) FROM ClientStats) AS unique_programs,
    (SELECT MAX(connections_per_host) FROM HostStats) AS top_host_connection_count,
    (SELECT MAX(connections_per_program) FROM ProgramStats) AS top_program_connection_count,
    (SELECT COUNT(*) FROM HostStats WHERE programs_per_host > 1) AS hosts_with_multiple_programs,
    (SELECT COUNT(*) FROM ProgramStats WHERE hosts_per_program > 1) AS programs_from_multiple_hosts`

// UserConnectionClientSummaryQueryAzureMI returns the client summary query for Azure SQL Managed Instance
const UserConnectionClientSummaryQueryAzureMI = `WITH ClientStats AS (
    SELECT 
        host_name,
        program_name,
        COUNT(session_id) AS connection_count
    FROM sys.dm_exec_sessions WITH (NOLOCK)
    WHERE is_user_process = 1
      AND host_name IS NOT NULL
      AND program_name IS NOT NULL
    GROUP BY host_name, program_name
),
HostStats AS (
    SELECT 
        host_name,
        COUNT(DISTINCT program_name) AS programs_per_host,
        SUM(connection_count) AS connections_per_host
    FROM ClientStats
    GROUP BY host_name
),
ProgramStats AS (
    SELECT 
        program_name,
        COUNT(DISTINCT host_name) AS hosts_per_program,
        SUM(connection_count) AS connections_per_program
    FROM ClientStats
    GROUP BY program_name
)
SELECT 
    (SELECT COUNT(DISTINCT host_name) FROM ClientStats) AS unique_hosts,
    (SELECT COUNT(DISTINCT program_name) FROM ClientStats) AS unique_programs,
    (SELECT MAX(connections_per_host) FROM HostStats) AS top_host_connection_count,
    (SELECT MAX(connections_per_program) FROM ProgramStats) AS top_program_connection_count,
    (SELECT COUNT(*) FROM HostStats WHERE programs_per_host > 1) AS hosts_with_multiple_programs,
    (SELECT COUNT(*) FROM ProgramStats WHERE hosts_per_program > 1) AS programs_from_multiple_hosts`

// LoginLogoutQuery returns the SQL query for login and logout rate metrics
// This query retrieves authentication activity counters from performance counters
//
// The query returns:
// - counter_name: Name of the performance counter (Logins/sec, Logouts/sec)
// - cntr_value: Current counter value representing rate per second
//
// These metrics are useful for:
// - Monitoring authentication activity and connection patterns
// - Detecting abnormal login/logout spikes that may indicate issues
// - Understanding connection churn and application behavior
// - Identifying potential security events or authentication problems
const LoginLogoutQuery = `SELECT
    RTRIM(counter_name) AS counter_name,
    cntr_value
FROM sys.dm_os_performance_counters WITH (NOLOCK)
WHERE object_name LIKE '%General Statistics%'
    AND counter_name IN ('Logins/sec', 'Logouts/sec')`

// LoginLogoutQueryAzureSQL returns the same query for Azure SQL Database
const LoginLogoutQueryAzureSQL = `SELECT
    RTRIM(counter_name) AS counter_name,
    cntr_value
FROM sys.dm_os_performance_counters
WHERE object_name LIKE '%General Statistics%'
    AND counter_name IN ('Logins/sec', 'Logouts/sec')`

// LoginLogoutQueryAzureMI returns the same query for Azure SQL Managed Instance
const LoginLogoutQueryAzureMI = `SELECT
    RTRIM(counter_name) AS counter_name,
    cntr_value
FROM sys.dm_os_performance_counters WITH (NOLOCK)
WHERE object_name LIKE '%General Statistics%'
    AND counter_name IN ('Logins/sec', 'Logouts/sec')`

// LoginLogoutSummaryQuery returns aggregated login/logout statistics
// This query provides summary metrics for authentication activity analysis
//
// The query returns:
// - logins_per_sec: Current login rate per second
// - logouts_per_sec: Current logout rate per second
// - total_auth_activity: Sum of logins and logouts per second
// - connection_churn_rate: Percentage of connections that are being churned
const LoginLogoutSummaryQuery = `WITH AuthStats AS (
    SELECT
        CASE WHEN counter_name = 'Logins/sec' THEN cntr_value ELSE 0 END AS logins_per_sec,
        CASE WHEN counter_name = 'Logouts/sec' THEN cntr_value ELSE 0 END AS logouts_per_sec
    FROM sys.dm_os_performance_counters WITH (NOLOCK)
    WHERE object_name LIKE '%General Statistics%'
        AND counter_name IN ('Logins/sec', 'Logouts/sec')
)
SELECT 
    MAX(logins_per_sec) AS logins_per_sec,
    MAX(logouts_per_sec) AS logouts_per_sec,
    (MAX(logins_per_sec) + MAX(logouts_per_sec)) AS total_auth_activity,
    CASE 
        WHEN MAX(logins_per_sec) > 0 
        THEN (MAX(logouts_per_sec) * 100.0) / MAX(logins_per_sec)
        ELSE 0 
    END AS connection_churn_rate
FROM AuthStats`

// LoginLogoutSummaryQueryAzureSQL returns the summary query for Azure SQL Database
const LoginLogoutSummaryQueryAzureSQL = `WITH AuthStats AS (
    SELECT
        CASE WHEN counter_name = 'Logins/sec' THEN cntr_value ELSE 0 END AS logins_per_sec,
        CASE WHEN counter_name = 'Logouts/sec' THEN cntr_value ELSE 0 END AS logouts_per_sec
    FROM sys.dm_os_performance_counters
    WHERE object_name LIKE '%General Statistics%'
        AND counter_name IN ('Logins/sec', 'Logouts/sec')
)
SELECT 
    MAX(logins_per_sec) AS logins_per_sec,
    MAX(logouts_per_sec) AS logouts_per_sec,
    (MAX(logins_per_sec) + MAX(logouts_per_sec)) AS total_auth_activity,
    CASE 
        WHEN MAX(logins_per_sec) > 0 
        THEN (MAX(logouts_per_sec) * 100.0) / MAX(logins_per_sec)
        ELSE 0 
    END AS connection_churn_rate
FROM AuthStats`

// LoginLogoutSummaryQueryAzureMI returns the summary query for Azure SQL Managed Instance
const LoginLogoutSummaryQueryAzureMI = `WITH AuthStats AS (
    SELECT
        CASE WHEN counter_name = 'Logins/sec' THEN cntr_value ELSE 0 END AS logins_per_sec,
        CASE WHEN counter_name = 'Logouts/sec' THEN cntr_value ELSE 0 END AS logouts_per_sec
    FROM sys.dm_os_performance_counters WITH (NOLOCK)
    WHERE object_name LIKE '%General Statistics%'
        AND counter_name IN ('Logins/sec', 'Logouts/sec')
)
SELECT 
    MAX(logins_per_sec) AS logins_per_sec,
    MAX(logouts_per_sec) AS logouts_per_sec,
    (MAX(logins_per_sec) + MAX(logouts_per_sec)) AS total_auth_activity,
    CASE 
        WHEN MAX(logins_per_sec) > 0 
        THEN (MAX(logouts_per_sec) * 100.0) / MAX(logins_per_sec)
        ELSE 0 
    END AS connection_churn_rate
FROM AuthStats`

// FailedLoginQuery returns the SQL query for failed login attempts from error log
// This query reads the SQL Server Error Log and filters for "Login failed" messages
// which is critical for security and connectivity troubleshooting
const FailedLoginQuery = `EXEC sp_readerrorlog 0, 1, 'Login failed'`

// FailedLoginQueryAzureSQL returns the failed login query for Azure SQL Database
// Azure SQL Database uses sys.event_log to track connection failures and authentication events
const FailedLoginQueryAzureSQL = `SELECT
    event_type,
    event_subtype_desc AS description,
    start_time,
    JSON_VALUE(CAST(additional_data AS NVARCHAR(MAX)), '$.client_ip') AS client_ip
FROM
    sys.event_log
WHERE
    event_type IN ('connection_failed')
    AND start_time >= DATEADD(HOUR, -24, GETUTCDATE())
ORDER BY
    start_time DESC`

// FailedLoginQueryAzureMI returns the failed login query for Azure SQL Managed Instance
const FailedLoginQueryAzureMI = `EXEC sp_readerrorlog 0, 1, 'Login failed'`

// FailedLoginSummaryQuery returns aggregated statistics about failed login attempts
// This query analyzes the SQL Server error log for failed login patterns and statistics
const FailedLoginSummaryQuery = `
DECLARE @FailedLogins TABLE (
    LogDate DATETIME,
    ProcessInfo NVARCHAR(100),
    Text NVARCHAR(MAX)
);

-- Insert data from the current and previous error logs
-- This captures "Login failed" messages
INSERT INTO @FailedLogins (LogDate, ProcessInfo, Text)
EXEC sp_readerrorlog 0, 1, 'Login failed';

INSERT INTO @FailedLogins (LogDate, ProcessInfo, Text)
EXEC sp_readerrorlog 1, 1, 'Login failed';

-- Use a Common Table Expression (CTE) to filter and parse the log text
WITH FilteredLogins AS (
    SELECT
        LogDate,
        -- Parse the username from the log text
        LTRIM(RTRIM(SUBSTRING(
            Text,
            CHARINDEX('user ''', Text) + 6,
            CHARINDEX('''', Text, CHARINDEX('user ''', Text) + 6) - (CHARINDEX('user ''', Text) + 6)
        ))) AS failed_user,
        -- Parse the client IP address from the log text
        LTRIM(RTRIM(SUBSTRING(
            Text,
            CHARINDEX('[CLIENT: ', Text) + 9,
            CHARINDEX(']', Text, CHARINDEX('[CLIENT: ', Text)) - (CHARINDEX('[CLIENT: ', Text) + 9)
        ))) AS source_ip
    FROM @FailedLogins
    WHERE Text LIKE '%Login failed for user%' -- Ensure we only process relevant rows
)
-- Aggregate the final metrics
SELECT
    COUNT(*) AS total_failed_logins,
    SUM(CASE WHEN LogDate >= DATEADD(HOUR, -1, GETDATE()) THEN 1 ELSE 0 END) AS recent_failed_logins,
    COUNT(DISTINCT failed_user) AS unique_failed_users,
    COUNT(DISTINCT source_ip) AS unique_failed_sources
FROM FilteredLogins`

// FailedLoginSummaryQueryAzureSQL returns summary statistics for Azure SQL Database using sys.event_log
// This query aggregates connection failure events from the event log for monitoring purposes
const FailedLoginSummaryQueryAzureSQL = `SELECT 
    COUNT(*) AS total_failed_logins,
    SUM(CASE WHEN start_time >= DATEADD(HOUR, -1, GETUTCDATE()) THEN 1 ELSE 0 END) AS recent_failed_logins,
    COUNT(DISTINCT JSON_VALUE(CAST(additional_data AS NVARCHAR(MAX)), '$.client_ip')) AS unique_failed_sources,
    COUNT(DISTINCT JSON_VALUE(CAST(additional_data AS NVARCHAR(MAX)), '$.username')) AS unique_failed_users
FROM sys.event_log
WHERE event_type IN ('connection_failed')
    AND start_time >= DATEADD(HOUR, -24, GETUTCDATE())`

// FailedLoginSummaryQueryAzureMI returns the summary query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports sp_readerrorlog with full functionality
const FailedLoginSummaryQueryAzureMI = `
DECLARE @FailedLogins TABLE (
    LogDate DATETIME,
    ProcessInfo NVARCHAR(100),
    Text NVARCHAR(MAX)
);

-- Insert data from the current and previous error logs
-- This captures "Login failed" messages
INSERT INTO @FailedLogins (LogDate, ProcessInfo, Text)
EXEC sp_readerrorlog 0, 1, 'Login failed';

INSERT INTO @FailedLogins (LogDate, ProcessInfo, Text)
EXEC sp_readerrorlog 1, 1, 'Login failed';

-- Use a Common Table Expression (CTE) to filter and parse the log text
WITH FilteredLogins AS (
    SELECT
        LogDate,
        -- Parse the username from the log text
        LTRIM(RTRIM(SUBSTRING(
            Text,
            CHARINDEX('user ''', Text) + 6,
            CHARINDEX('''', Text, CHARINDEX('user ''', Text) + 6) - (CHARINDEX('user ''', Text) + 6)
        ))) AS failed_user,
        -- Parse the client IP address from the log text
        LTRIM(RTRIM(SUBSTRING(
            Text,
            CHARINDEX('[CLIENT: ', Text) + 9,
            CHARINDEX(']', Text, CHARINDEX('[CLIENT: ', Text)) - (CHARINDEX('[CLIENT: ', Text) + 9)
        ))) AS source_ip
    FROM @FailedLogins
    WHERE Text LIKE '%Login failed for user%' -- Ensure we only process relevant rows
)
-- Aggregate the final metrics
SELECT
    COUNT(*) AS total_failed_logins,
    SUM(CASE WHEN LogDate >= DATEADD(HOUR, -1, GETDATE()) THEN 1 ELSE 0 END) AS recent_failed_logins,
    COUNT(DISTINCT failed_user) AS unique_failed_users,
    COUNT(DISTINCT source_ip) AS unique_failed_sources
FROM FilteredLogins`
