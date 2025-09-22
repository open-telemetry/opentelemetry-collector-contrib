# Oracle Scrapers

This folder contains organized scraper functions for different categories of Oracle metrics.

## Organization

### 📊 **session_scraper.go**
- **Purpose**: Session-related metrics
- **Metrics**:
  - `scrapeSessionCount()` - Total user sessions
  - `scrapeActiveSessionCount()` - Active sessions 
  - `scrapeInactiveSessionCount()` - Inactive sessions
  - `scrapeBlockedSessionCount()` - Blocked sessions
- **SQL Views**: `v$session`

### 💾 **tablespace_scraper.go**
- **Purpose**: Storage and tablespace metrics
- **Metrics**:
  - `scrapeTablespaceMetrics()` - Tablespace size and limits
  - `scrapeTablespaceUsageMetrics()` - Tablespace usage percentages
  - `scrapeTempTablespaceMetrics()` - Temporary tablespace metrics
- **SQL Views**: `dba_data_files`, `dba_tablespace_usage_metrics`, `v$temp_space_header`

### ⚡ **performance_scraper.go**
- **Purpose**: Performance and system metrics
- **Metrics**:
  - `scrapeCpuUsage()` - CPU usage statistics
  - `scrapeMemoryMetrics()` - Memory usage metrics
  - `scrapeWaitEvents()` - Wait event statistics
  - `scrapeProcessCount()` - Oracle process count
  - `scrapeRedoLogSwitches()` - Redo log switch frequency
- **SQL Views**: `v$sysstat`, `v$system_event`, `v$process`, `v$log_history`

### 🗄️ **database_scraper.go**
- **Purpose**: Database-level metrics and administration
- **Metrics**:
  - `scrapeDatabaseSize()` - Total database size
  - `scrapeUserCount()` - Active user accounts
  - `scrapeLockCount()` - Database locks
  - `scrapeArchiveLogCount()` - Archive log generation
  - `scrapeInvalidObjectsCount()` - Invalid database objects
- **SQL Views**: `dba_data_files`, `dba_users`, `v$lock`, `v$archived_log`, `dba_objects`

## Usage

All scraper functions are called from the main `scrape()` function in `../scraper.go`:

```go
// Session-related metrics
scrapeErrors = append(scrapeErrors, s.scrapeSessionCount(ctx)...)
scrapeErrors = append(scrapeErrors, s.scrapeActiveSessionCount(ctx)...)
// ... etc

// Tablespace-related metrics  
scrapeErrors = append(scrapeErrors, s.scrapeTablespaceMetrics(ctx)...)
// ... etc

// Performance-related metrics
scrapeErrors = append(scrapeErrors, s.scrapeCpuUsage(ctx)...)
// ... etc

// Database-related metrics
scrapeErrors = append(scrapeErrors, s.scrapeDatabaseSize(ctx)...)
// ... etc
```

## Adding New Metrics

To add new metrics:

1. **Choose the appropriate scraper file** based on the metric category
2. **Add the SQL query constant** at the top of the file
3. **Implement the scraper function** following the existing pattern
4. **Add the metric definition** to `../metadata.yaml`
5. **Run `go generate`** to update metadata
6. **Call the function** from the main `scrape()` method
7. **Uncomment the metric recording line** once the metric is defined

## Notes

- Each scraper function returns `[]error` for partial failure handling
- Most scrapers reuse existing database clients for efficiency
- Metric recording calls are commented out - uncomment after adding to metadata.yaml
- All scrapers follow the same error handling and logging patterns
