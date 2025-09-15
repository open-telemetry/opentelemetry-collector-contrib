# New Relic SQL Server Receiver

The New Relic SQL Server Receiver is an OpenTelemetry receiver that collects metrics from Microsoft SQL Server instances. It is based on New Relic's proven nri-mssql integration patterns and provides comprehensive monitoring capabilities for SQL Server environments.

## Features

### Core Instance Metrics
- User connections count
- SQL compilations and recompilations per second
- Buffer pool statistics and page life expectancy
- Deadlock detection and user errors
- Transaction and checkpoint activity

### Query Performance Analytics
- Blocking session detection and analysis
- Slow query identification
- Query performance monitoring
- Resource wait analysis

### Authentication Support
- SQL Server authentication (Windows Auth, SQL Auth)
- Azure AD Service Principal authentication
- SSL/TLS connection encryption

### Environment Compatibility
- SQL Server 2012 and later versions
- Azure SQL Database
- Azure SQL Managed Instance
- SQL Server on Linux

## Configuration

### Basic Configuration

```yaml
receivers:
  newrelicsqlserverreceiver:
    hostname: "localhost"
    port: "1433"
    username: "monitoring_user"
    password: "secure_password"
    collection_interval: 15s
```

### Full Configuration Options

```yaml
receivers:
  newrelicsqlserverreceiver:
    # Connection settings
    hostname: "sql-server.example.com"    # Required: SQL Server hostname
    port: "1433"                          # Port number (use either port or instance)
    instance: "MSSQLSERVER"               # Instance name (use either port or instance)
    username: "monitor_user"              # SQL Server username
    password: "secure_password"           # SQL Server password
    
    # Azure AD Authentication (alternative to username/password)
    client_id: "azure-client-id"          # Azure AD Service Principal Client ID
    tenant_id: "azure-tenant-id"          # Azure AD Tenant ID  
    client_secret: "azure-client-secret"  # Azure AD Service Principal Secret
    
    # SSL Configuration
    enable_ssl: false                     # Enable SSL/TLS encryption
    trust_server_certificate: false      # Trust server certificate without validation
    certificate_location: "/path/to/cert.pem"  # Path to certificate file
    
    # Collection settings
    collection_interval: 15s              # How often to collect metrics
    timeout: 30s                         # Query timeout duration
    max_concurrent_workers: 10            # Maximum concurrent database connections
    
    # Feature toggles
    enable_buffer_metrics: true           # Collect buffer pool metrics
    enable_database_reserve_metrics: true # Collect database space metrics
    enable_disk_metrics_in_bytes: true   # Collect disk usage in bytes
    
    # Query monitoring
    enable_query_monitoring: false        # Enable query performance analysis
    query_monitoring_response_time_threshold: 1000  # Slow query threshold (ms)
    query_monitoring_count_threshold: 20   # Max queries in analysis results
    query_monitoring_fetch_interval: 15   # Query analysis collection interval (seconds)
    
    # Custom queries
    custom_metrics_query: "SELECT 'my_metric' as metric_name, 123 as metric_value, 'gauge' as metric_type"
    custom_metrics_config: "/path/to/custom_queries.yaml"  # Path to custom queries file
    
    # Advanced connection settings
    extra_connection_url_args: "applicationintent=readonly&encrypt=true"  # Additional connection parameters
```

## Metrics

| Metric Name | Description | Unit | Type |
|-------------|-------------|------|------|
| `sqlserver.stats.connections` | Number of user connections | {connections} | Gauge |
| `sqlserver.stats.sql_compilations` | SQL compilations per second | {compilations}/s | Gauge |
| `sqlserver.stats.sql_recompilations` | SQL recompilations per second | {recompilations}/s | Gauge |
| `sqlserver.buffer.checkpoint_pages` | Checkpoint pages per second | {pages}/s | Gauge |
| `sqlserver.access.page_splits` | Page splits per second | {page_splits}/s | Gauge |
| `sqlserver.stats.deadlocks` | Deadlocks per second | {deadlocks}/s | Gauge |
| `sqlserver.bufferpool.page_life_expectancy` | Page life expectancy | ms | Gauge |
| `sqlserver.instance.transactions` | Transactions per second | {transactions}/s | Gauge |
| `sqlserver.query.blocked_sessions` | Number of blocked sessions | {sessions} | Gauge |
| `sqlserver.query.blocking_sessions` | Number of sessions causing blocks | {sessions} | Gauge |
| `sqlserver.query.max_blocking_wait_time` | Maximum blocking wait time | ms | Gauge |

## Resource Attributes

| Attribute | Description |
|-----------|-------------|
| `server.address` | SQL Server hostname or IP address |
| `server.port` | SQL Server port number |
| `sqlserver.instance.name` | SQL Server instance name |
| `sqlserver.database.name` | Database name (for database-specific metrics) |
| `db.system` | Database system identifier (`mssql`) |
| `service.name` | Service name (`sqlserver`) |

## Authentication Methods

### SQL Server Authentication
Standard SQL Server authentication using username and password:

```yaml
receivers:
  newrelicsqlserverreceiver:
    username: "monitoring_user"
    password: "secure_password"
```

### Azure AD Service Principal
For Azure SQL Database or Managed Instance:

```yaml
receivers:
  newrelicsqlserverreceiver:
    client_id: "12345678-1234-1234-1234-123456789012"
    tenant_id: "87654321-4321-4321-4321-210987654321"
    client_secret: "your-client-secret"
```

### Windows Authentication
For Windows-based SQL Server instances (connection string based):

```yaml
receivers:
  newrelicsqlserverreceiver:
    extra_connection_url_args: "integrated security=SSPI"
```

## Prerequisites

### SQL Server Permissions
The monitoring user requires the following minimum permissions:

```sql
-- Grant VIEW SERVER STATE for instance-level metrics
GRANT VIEW SERVER STATE TO [monitoring_user];

-- Grant VIEW DATABASE STATE for database-specific metrics
USE [database_name];
GRANT VIEW DATABASE STATE TO [monitoring_user];

-- For custom queries, grant appropriate SELECT permissions
GRANT SELECT ON [specific_tables] TO [monitoring_user];
```

### Network Access
- Ensure the collector can reach the SQL Server instance on the configured port
- For Azure SQL, ensure firewall rules allow collector IP addresses
- For SSL connections, ensure proper certificates are configured

## Custom Queries

You can define custom metrics using SQL queries. Create a YAML configuration file:

```yaml
# custom_queries.yaml
custom_queries:
  - query: |
      SELECT 
        'database_size' as metric_name,
        SUM(size * 8.0 / 1024) as metric_value,
        'gauge' as metric_type,
        DB_NAME() as database_name
      FROM sys.database_files
    databases: ["MyDatabase"]
    
  - query: |
      SELECT 
        'active_transactions' as metric_name,
        COUNT(*) as metric_value,
        'gauge' as metric_type
      FROM sys.dm_tran_active_transactions
```

Then reference it in your collector configuration:

```yaml
receivers:
  newrelicsqlserverreceiver:
    custom_metrics_config: "/path/to/custom_queries.yaml"
```

## Troubleshooting

### Connection Issues
1. Verify SQL Server is running and accessible
2. Check firewall rules and network connectivity
3. Validate authentication credentials
4. Test connection string parameters

### Permission Errors
1. Ensure monitoring user has required VIEW permissions
2. Check database-specific permissions for database metrics
3. Verify sys.dm_* views are accessible

### Query Monitoring Issues
1. Ensure `enable_query_monitoring` is set to `true`
2. Check that Query Store is enabled (SQL Server 2016+)
3. Verify sys.dm_exec_* views are accessible
4. Adjust `query_monitoring_response_time_threshold` as needed

### Performance Considerations
1. Use `max_concurrent_workers` to limit database connections
2. Adjust `collection_interval` based on your monitoring needs
3. For high-volume environments, consider increasing `timeout` values
4. Monitor collector resource usage and SQL Server impact

## Based on New Relic Integration

This receiver is built using patterns and methodologies from New Relic's proven nri-mssql integration, providing:

- Battle-tested SQL queries for reliable metric collection
- Robust error handling and connection management
- Comprehensive coverage of SQL Server monitoring scenarios
- Performance-optimized data collection strategies

For more information about the original New Relic integration, see:
https://docs.newrelic.com/docs/infrastructure/host-integrations/host-integrations-list/microsoft-sql-server-monitoring-integration/
