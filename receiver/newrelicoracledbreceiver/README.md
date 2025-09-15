# New Relic Oracle DB Receiver

The New Relic Oracle DB Receiver is an OpenTelemetry receiver that collects comprehensive metrics from Oracle databases, inspired by New Relic's Oracle monitoring capabilities.

## 🏗️ **Modular Architecture**

This receiver features a **modular metrics architecture** for enhanced maintainability and scalability:

### **Metrics Organization**
- **`config.go`** (124 lines) - Configuration types and defaults
- **`performance_metrics.go`** (303 lines) - CPU, executions, parsing metrics  
- **`io_metrics.go`** (297 lines) - Physical I/O and block operations
- **`memory_metrics.go`** (148 lines) - PGA, SGA, shared pool metrics
- **`cache_metrics.go`** (38 lines) - Buffer and library cache hit ratios
- **`generated_metrics.go`** (685 lines) - Core orchestration + remaining metrics

**Total Reduction**: 63% decrease from original 1,863-line monolithic file to organized category-based structure.

## Features

This receiver provides comprehensive Oracle database monitoring with the following capabilities:

### Core Metrics Collection
- **CPU and Execution Metrics**: CPU time, executions, parse operations
- **Memory Metrics**: PGA and SGA memory usage, buffer cache hit ratios
- **I/O Performance**: Physical reads/writes, I/O requests, logical reads
- **Session Management**: Active sessions by status and type
- **Transaction Metrics**: User commits, rollbacks, logon counts
- **Lock and Concurrency**: Deadlock detection, DML locks, enqueue locks
- **Tablespace Management**: Usage, free space, offline status

### New Relic-Inspired Features
- **Query Performance Monitoring**: Top queries by CPU time, execution statistics
- **Wait Event Analysis**: Database wait events and timing
- **Long-Running Query Detection**: Queries exceeding configurable thresholds
- **Parallel Operations Tracking**: Parallel query execution metrics
- **Account Security**: Locked account monitoring
- **Advanced Diagnostics**: Rollback segment metrics, redo log analysis

### Query Performance Insights
- SQL execution statistics with obfuscated query text
- CPU time, elapsed time, and wait time breakdown
- Physical and logical I/O metrics per query
- Buffer cache usage and disk operations
- Query plan hash values for performance correlation

## Configuration

### Basic Configuration

```yaml
receivers:
  newrelicoracledb:
    # Connection configuration
    endpoint: "localhost:1521"
    service: "ORCL"
    username: "monitoring_user"
    password: "password"
    
    # Collection interval
    collection_interval: 60s
    
    # New Relic-style configuration
    new_relic_style_config:
      enable_custom_queries: false
      enable_tablespace_metrics: true
      enable_session_metrics: true
      enable_wait_event_metrics: false
      enable_lock_metrics: true
      query_timeout: 30s
      max_sql_length: 4000
      obfuscated_queries: true
    
    # Query performance monitoring
    top_query_collection:
      enabled: false
      top_n: 20
      collection_window: 10m
      min_execution_time: 1s
      include_query_text: true
    
    # Query sampling
    query_sample_collection:
      enabled: false
      sample_rate: 0.1
      explain_plans: false
      max_samples_per_minute: 100
    
    # Tablespace monitoring
    tablespace_config:
      enabled: true
      include_system_tablespaces: false
      threshold_percentage: 80.0
```

### Advanced Configuration

```yaml
receivers:
  newrelicoracledb:
    # Full connection string (alternative to individual fields)
    datasource: "oracle://user:password@host:port/service"
    
    # Extended monitoring
    new_relic_style_config:
      enable_custom_queries: true
      enable_tablespace_metrics: true
      enable_session_metrics: true
      enable_wait_event_metrics: true
      enable_lock_metrics: true
      enable_parallel_metrics: true
      enable_rollback_metrics: true
      query_timeout: 45s
      max_sql_length: 8000
      obfuscated_queries: true
      include_system_sessions: false
    
    # Performance monitoring
    top_query_collection:
      enabled: true
      top_n: 50
      collection_window: 5m
      min_execution_time: 500ms
      include_query_text: true
      include_execution_plans: false
    
    # Enhanced query sampling
    query_sample_collection:
      enabled: true
      sample_rate: 0.05
      explain_plans: true
      max_samples_per_minute: 200
      include_bind_variables: false
```

## Supported Metrics

### System Performance Metrics
- `newrelicoracledb.cpu_time` - Cumulative CPU time in seconds
- `newrelicoracledb.executions` - Total SQL executions
- `newrelicoracledb.hard_parses` - Hard parse operations
- `newrelicoracledb.parse_calls` - Total parse calls

### Memory Metrics
- `newrelicoracledb.pga_memory` - Program Global Area memory usage
- `newrelicoracledb.sga_hit_ratio` - Buffer cache hit ratio
- `newrelicoracledb.shared_pool_free` - Shared pool free memory percentage

### I/O Performance Metrics
- `newrelicoracledb.physical_reads` - Physical disk reads
- `newrelicoracledb.physical_writes` - Physical disk writes
- `newrelicoracledb.physical_read_io_requests` - Physical read I/O requests
- `newrelicoracledb.physical_write_io_requests` - Physical write I/O requests
- `newrelicoracledb.logical_reads` - Session logical reads

### Session and Connection Metrics
- `newrelicoracledb.sessions` - Active sessions by type and status
- `newrelicoracledb.logons` - Cumulative logon count
- `newrelicoracledb.user_commits` - User transaction commits
- `newrelicoracledb.user_rollbacks` - User transaction rollbacks

### Lock and Concurrency Metrics
- `newrelicoracledb.enqueue_deadlocks` - Table/row lock deadlocks
- `newrelicoracledb.dml_locks_usage` - Active DML locks
- `newrelicoracledb.enqueue_locks_usage` - Active enqueue locks

### Tablespace Metrics
- `newrelicoracledb.tablespace_size_usage` - Tablespace usage in bytes
- `newrelicoracledb.tablespace_usage_percentage` - Tablespace usage percentage
- `newrelicoracledb.tablespace_offline` - Tablespace offline status

### Query Performance Metrics (Logs)
- `newrelicoracledb.query_cpu_time` - CPU time per query
- `newrelicoracledb.query_elapsed_time` - Total elapsed time per query
- `newrelicoracledb.query_executions` - Execution count per query
- `newrelicoracledb.query_physical_read_requests` - Physical reads per query

## Resource Attributes

- `newrelicoracledb.instance.name` - Oracle instance name
- `host.name` - Database server hostname
- `newrelicoracledb.database.name` - Oracle database name

## Attribute Dimensions

### Session Attributes
- `session_status` - Session status (ACTIVE, INACTIVE, etc.)
- `session_type` - Session type (USER, BACKGROUND)

### Tablespace Attributes
- `tablespace_name` - Tablespace name
- `instance_id` - Oracle instance identifier

### Query Performance Attributes
- `newrelicoracledb.sql_id` - SQL identifier
- `newrelicoracledb.query_plan` - Query execution plan
- `db.query.text` - Obfuscated query text

## Prerequisites

### Database Permissions

The monitoring user needs the following permissions:

```sql
-- Create monitoring user
CREATE USER monitoring_user IDENTIFIED BY "password";

-- Grant necessary privileges
GRANT CREATE SESSION TO monitoring_user;
GRANT SELECT ON v_$sysstat TO monitoring_user;
GRANT SELECT ON v_$session TO monitoring_user;
GRANT SELECT ON v_$pgastat TO monitoring_user;
GRANT SELECT ON v_$sgainfo TO monitoring_user;
GRANT SELECT ON v_$sgastat TO monitoring_user;
GRANT SELECT ON v_$resource_limit TO monitoring_user;
GRANT SELECT ON v_$system_event TO monitoring_user;
GRANT SELECT ON v_$sql TO monitoring_user;
GRANT SELECT ON v_$sqltext TO monitoring_user;
GRANT SELECT ON v_$instance TO monitoring_user;
GRANT SELECT ON dba_data_files TO monitoring_user;
GRANT SELECT ON dba_free_space TO monitoring_user;
GRANT SELECT ON dba_tablespaces TO monitoring_user;
GRANT SELECT ON dba_users TO monitoring_user;

-- For advanced monitoring (optional)
GRANT SELECT ON v_$sql_plan TO monitoring_user;
GRANT SELECT ON v_$session_wait TO monitoring_user;
GRANT SELECT ON v_$session_event TO monitoring_user;
```

### Oracle Client Libraries

This receiver uses the `github.com/sijms/go-ora/v2` driver, which is a pure Go Oracle client that doesn't require Oracle client libraries.

## Security Considerations

- **Password Security**: Use environment variables or secure configuration management for passwords
- **Query Obfuscation**: Enable `obfuscated_queries` to prevent sensitive data exposure in logs
- **Limited Permissions**: Grant only necessary database permissions to the monitoring user
- **Network Security**: Use encrypted connections when possible

## Performance Impact

- **Low Overhead**: Optimized queries with minimal database impact
- **Configurable Collection**: Adjust collection intervals based on your monitoring needs
- **Query Sampling**: Use sampling to reduce performance impact on high-traffic systems
- **Connection Pooling**: Efficient connection management to minimize database load

## Comparison with Other Integrations

This receiver combines the best features from:
- **OpenTelemetry Oracle Receiver**: Standard OpenTelemetry metrics collection
- **New Relic Oracle Integration**: Comprehensive monitoring and query performance insights
- **Datadog Oracle Integration**: Advanced analytics and custom metrics

Key differentiators:
- Native OpenTelemetry integration with full semantic conventions
- New Relic-inspired query performance monitoring
- Comprehensive tablespace and session management metrics
- Advanced lock and concurrency monitoring
- Built-in query obfuscation and security features

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Verify database connectivity and credentials
   - Check Oracle listener status and port accessibility
   - Ensure monitoring user has necessary permissions

2. **Missing Metrics**
   - Verify database user has SELECT permissions on system views
   - Check if Oracle instance is running and accessible
   - Review receiver configuration for enabled metric groups

3. **Performance Issues**
   - Increase collection interval for high-load systems
   - Disable advanced features like query sampling if not needed
   - Monitor database load and adjust query timeouts

### Debug Configuration

```yaml
receivers:
  newrelicoracledb:
    # ... other config ...
    
    # Enable debug logging
    debug: true
    
    # Reduce timeout for testing
    query_timeout: 10s
    
    # Minimal metric collection for debugging
    new_relic_style_config:
      enable_custom_queries: false
      enable_wait_event_metrics: false
```

## Contributing

This receiver is part of the OpenTelemetry Collector Contrib project. Contributions are welcome! Please see the [contributing guidelines](../../CONTRIBUTING.md) for more information.

## License

This receiver is licensed under the Apache License 2.0. See [LICENSE](../../LICENSE) for more information.
