// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoracledbreceiver"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
)

var (
	errBadDataSource       = errors.New("datasource is invalid")
	errBadEndpoint         = errors.New("endpoint must be specified as host:port")
	errBadPort             = errors.New("invalid port in endpoint")
	errEmptyEndpoint       = errors.New("endpoint must be specified")
	errEmptyPassword       = errors.New("password must be set")
	errEmptyService        = errors.New("service must be specified")
	errEmptyUsername       = errors.New("username must be set")
	errMaxQuerySampleCount = errors.New("`max_query_sample_count` must be between 1 and 10000")
	errTopQueryCount       = errors.New("`top_query_count` must be between 1 and 200 and less than or equal to `max_query_sample_count`")
	errMaxOpenConnections  = errors.New("`max_open_connections` must be between 1 and 100")
)

// ExtendedConfig represents extended configuration options
type ExtendedConfig struct {
	ExtendedMetrics       bool `mapstructure:"extended_metrics"`
	MaxOpenConnections    int  `mapstructure:"max_open_connections"`
	DisableConnectionPool bool `mapstructure:"disable_connection_pool"`

	// Custom query configuration
	CustomMetricsQuery  string `mapstructure:"custom_metrics_query"`
	CustomMetricsConfig string `mapstructure:"custom_metrics_config"`

	// Security settings
	IsSysDBA  bool `mapstructure:"is_sys_dba"`
	IsSysOper bool `mapstructure:"is_sys_oper"`

	// Multitenant support (NRI Oracle DB compatible)
	SysMetricsSource string `mapstructure:"sys_metrics_source"` // "PDB", "All", or default for CDB

	// Skip metrics groups (NRI Oracle DB compatible)
	SkipMetricsGroups []string `mapstructure:"skip_metrics_groups"`
}

// TablespaceConfig represents tablespace monitoring configuration
type TablespaceConfig struct {
	IncludeTablespaces []string `mapstructure:"include_tablespaces"`
	ExcludeTablespaces []string `mapstructure:"exclude_tablespaces"`
}

// TopQueryCollection represents query performance monitoring configuration
type TopQueryCollection struct {
	MaxQuerySampleCount uint `mapstructure:"max_query_sample_count"`
	TopQueryCount       uint `mapstructure:"top_query_count"`
}

// QuerySample represents query sampling configuration
type QuerySample struct {
	MaxRowsPerQuery uint64 `mapstructure:"max_rows_per_query"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// LogsConfig represents logs collection configuration
type LogsConfig struct {
	// Enable logs collection
	EnableLogs bool `mapstructure:"enable_logs"`

	// Log sources to collect
	CollectAlertLogs   bool `mapstructure:"collect_alert_logs"`
	CollectAuditLogs   bool `mapstructure:"collect_audit_logs"`
	CollectTraceFiles  bool `mapstructure:"collect_trace_files"`
	CollectArchiveLogs bool `mapstructure:"collect_archive_logs"`

	// File paths for log collection
	AlertLogPath   string `mapstructure:"alert_log_path"`   // Path to alert log directory
	AuditLogPath   string `mapstructure:"audit_log_path"`   // Path to audit log directory
	TraceLogPath   string `mapstructure:"trace_log_path"`   // Path to trace log directory
	ArchiveLogPath string `mapstructure:"archive_log_path"` // Path to archive log directory

	// Collection settings
	PollInterval    string   `mapstructure:"poll_interval"`     // How often to check for new logs
	MaxLogFileSize  int64    `mapstructure:"max_log_file_size"` // Maximum log file size to process (MB)
	LogFilePatterns []string `mapstructure:"log_file_patterns"` // File patterns to match
	ExcludePatterns []string `mapstructure:"exclude_patterns"`  // Patterns to exclude

	// Parsing settings
	ParseErrors        bool `mapstructure:"parse_errors"`         // Parse Oracle error codes
	ParseStackTraces   bool `mapstructure:"parse_stack_traces"`   // Parse stack traces from logs
	ParseSQLStatements bool `mapstructure:"parse_sql_statements"` // Extract SQL statements

	// Log level filtering
	MinLogLevel    string   `mapstructure:"min_log_level"`   // Minimum log level to collect
	ExcludeLevels  []string `mapstructure:"exclude_levels"`  // Log levels to exclude
	IncludeClasses []string `mapstructure:"include_classes"` // Oracle log classes to include
	ExcludeClasses []string `mapstructure:"exclude_classes"` // Oracle log classes to exclude

	// Database query-based log collection
	QueryBasedCollection bool   `mapstructure:"query_based_collection"` // Use SQL queries to collect logs
	AlertLogQuery        string `mapstructure:"alert_log_query"`        // SQL query for alert logs
	AuditLogQuery        string `mapstructure:"audit_log_query"`        // SQL query for audit logs

	// Advanced settings
	PreserveBinaryLogs bool `mapstructure:"preserve_binary_logs"` // Include binary log content
	MaxRetentionDays   int  `mapstructure:"max_retention_days"`   // Log retention period
	BatchSize          int  `mapstructure:"batch_size"`           // Number of log entries per batch

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config represents the receiver configuration
type Config struct {
	// Connection configuration (primary)
	DataSource string `mapstructure:"datasource"` // Complete connection string (takes precedence)

	// Connection components (if datasource not provided)
	Endpoint string `mapstructure:"endpoint"` // host:port format
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Service  string `mapstructure:"service"` // Oracle service name

	// Alternative connection format (NRI Oracle DB style)
	Hostname    string `mapstructure:"hostname"`     // Alternative to endpoint
	Port        string `mapstructure:"port"`         // Alternative to endpoint
	ServiceName string `mapstructure:"service_name"` // Alternative to service

	// OpenTelemetry configuration
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Extended configuration
	ExtendedConfig `mapstructure:",squash"`

	// Query and performance monitoring
	TopQueryCollection `mapstructure:"top_query_collection"`
	QuerySample        `mapstructure:"query_sample_collection"`
	TablespaceConfig   `mapstructure:"tablespace_config"`

	// Logs configuration
	LogsConfig `mapstructure:"logs"`
}

// Validate validates the configuration
func (c Config) Validate() error {
	var allErrs error

	// Validate connection configuration
	if err := c.validateConnection(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	// Validate New Relic style configuration
	if err := c.validateExtendedConfig(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	// Validate query configuration
	if err := c.validateQueryConfig(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	// Validate logs configuration
	if err := c.validateLogsConfig(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	return allErrs
}

// validateConnection validates database connection configuration
func (c Config) validateConnection() error {
	var allErrs error

	// If DataSource is defined it takes precedence over the rest of the connection options.
	if c.DataSource == "" {
		// Determine endpoint - prioritize Endpoint over Hostname:Port
		endpoint := c.Endpoint
		if endpoint == "" && c.Hostname != "" && c.Port != "" {
			endpoint = net.JoinHostPort(c.Hostname, c.Port)
		}

		if endpoint == "" {
			allErrs = multierr.Append(allErrs, errEmptyEndpoint)
		} else {
			host, portStr, err := net.SplitHostPort(endpoint)
			if err != nil {
				return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err.Error()))
			}

			if host == "" {
				allErrs = multierr.Append(allErrs, errBadEndpoint)
			}

			port, err := strconv.ParseInt(portStr, 10, 32)
			if err != nil {
				allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err.Error()))
			}

			if port < 0 || port > 65535 {
				allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %d", errBadPort, port))
			}
		}

		if c.Username == "" {
			allErrs = multierr.Append(allErrs, errEmptyUsername)
		}

		if c.Password == "" {
			allErrs = multierr.Append(allErrs, errEmptyPassword)
		}

		// Service validation - accept either Service or ServiceName
		service := c.Service
		if service == "" {
			service = c.ServiceName
		}
		if service == "" {
			allErrs = multierr.Append(allErrs, errEmptyService)
		}
	} else {
		if _, err := url.Parse(c.DataSource); err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadDataSource, err.Error()))
		}
	}

	return allErrs
}

// validateExtendedConfig validates extended configuration
func (c Config) validateExtendedConfig() error {
	var allErrs error

	// Validate max open connections
	if c.MaxOpenConnections < 1 || c.MaxOpenConnections > 100 {
		allErrs = multierr.Append(allErrs, errMaxOpenConnections)
	}

	// Validate SysMetricsSource (NRI Oracle DB compatible)
	if c.SysMetricsSource != "" {
		validSources := map[string]bool{
			"PDB": true, // Pluggable Database
			"All": true, // CDB & PDB containers
			"CDB": true, // Container Database only
		}
		if !validSources[c.SysMetricsSource] {
			allErrs = multierr.Append(allErrs, errors.New("sys_metrics_source must be one of: PDB, All, CDB"))
		}
	}

	// Validate privilege settings (cannot be both SYSDBA and SYSOPER)
	if c.IsSysDBA && c.IsSysOper {
		allErrs = multierr.Append(allErrs, errors.New("cannot be both SYSDBA and SYSOPER"))
	}

	return allErrs
}

// validateQueryConfig validates query monitoring configuration
func (c Config) validateQueryConfig() error {
	var allErrs error

	// Validate top query collection
	if c.TopQueryCollection.MaxQuerySampleCount < 1 || c.TopQueryCollection.MaxQuerySampleCount > 10000 {
		allErrs = multierr.Append(allErrs, errMaxQuerySampleCount)
	}

	if c.TopQueryCollection.TopQueryCount < 1 ||
		c.TopQueryCollection.TopQueryCount > 200 ||
		c.TopQueryCollection.TopQueryCount > c.TopQueryCollection.MaxQuerySampleCount {
		allErrs = multierr.Append(allErrs, errTopQueryCount)
	}

	// Validate custom query configuration
	if c.CustomMetricsQuery != "" && c.CustomMetricsConfig != "" {
		allErrs = multierr.Append(allErrs, errors.New("cannot specify both custom_metrics_query and custom_metrics_config"))
	}

	return allErrs
}

// GetConnectionString builds a connection string based on the configuration
// Follows NRI Oracle DB pattern for compatibility
func (c Config) GetConnectionString() string {
	// If DataSource is provided, use it directly
	if c.DataSource != "" {
		return c.DataSource
	}

	// Build from components - prioritize Endpoint over Hostname:Port
	endpoint := c.Endpoint
	if endpoint == "" && c.Hostname != "" && c.Port != "" {
		endpoint = net.JoinHostPort(c.Hostname, c.Port)
	}

	// Use Service or ServiceName
	service := c.Service
	if service == "" {
		service = c.ServiceName
	}

	// Build basic connection string: host:port/service
	connString := fmt.Sprintf("%s/%s", endpoint, service)

	return connString
}

// GetEffectiveEndpoint returns the effective endpoint (host:port)
func (c Config) GetEffectiveEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	if c.Hostname != "" && c.Port != "" {
		return net.JoinHostPort(c.Hostname, c.Port)
	}
	return ""
}

// GetEffectiveService returns the effective service name
func (c Config) GetEffectiveService() string {
	if c.Service != "" {
		return c.Service
	}
	return c.ServiceName
}

// validateLogsConfig validates logs collection configuration
func (c Config) validateLogsConfig() error {
	var allErrs error

	// If logs are not enabled, skip validation
	if !c.LogsConfig.EnableLogs {
		return nil
	}

	// Validate that at least one log source is enabled
	if !c.LogsConfig.CollectAlertLogs && !c.LogsConfig.CollectAuditLogs &&
		!c.LogsConfig.CollectTraceFiles && !c.LogsConfig.CollectArchiveLogs &&
		!c.LogsConfig.QueryBasedCollection {
		allErrs = multierr.Append(allErrs, errors.New("at least one log source must be enabled when logs collection is enabled"))
	}

	// Validate file paths if file-based collection is enabled
	if c.LogsConfig.CollectAlertLogs && c.LogsConfig.AlertLogPath == "" && !c.LogsConfig.QueryBasedCollection {
		allErrs = multierr.Append(allErrs, errors.New("alert_log_path must be specified when collect_alert_logs is enabled"))
	}

	if c.LogsConfig.CollectAuditLogs && c.LogsConfig.AuditLogPath == "" && !c.LogsConfig.QueryBasedCollection {
		allErrs = multierr.Append(allErrs, errors.New("audit_log_path must be specified when collect_audit_logs is enabled"))
	}

	if c.LogsConfig.CollectTraceFiles && c.LogsConfig.TraceLogPath == "" {
		allErrs = multierr.Append(allErrs, errors.New("trace_log_path must be specified when collect_trace_files is enabled"))
	}

	if c.LogsConfig.CollectArchiveLogs && c.LogsConfig.ArchiveLogPath == "" {
		allErrs = multierr.Append(allErrs, errors.New("archive_log_path must be specified when collect_archive_logs is enabled"))
	}

	// Validate numeric values
	if c.LogsConfig.MaxLogFileSize < 0 {
		allErrs = multierr.Append(allErrs, errors.New("max_log_file_size must be non-negative"))
	}

	if c.LogsConfig.MaxRetentionDays < 0 {
		allErrs = multierr.Append(allErrs, errors.New("max_retention_days must be non-negative"))
	}

	if c.LogsConfig.BatchSize <= 0 {
		allErrs = multierr.Append(allErrs, errors.New("batch_size must be positive"))
	}

	// Validate log level
	validLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", ""}
	if c.LogsConfig.MinLogLevel != "" {
		found := false
		for _, level := range validLevels {
			if c.LogsConfig.MinLogLevel == level {
				found = true
				break
			}
		}
		if !found {
			allErrs = multierr.Append(allErrs, fmt.Errorf("invalid min_log_level: %s, must be one of %v", c.LogsConfig.MinLogLevel, validLevels))
		}
	}

	return allErrs
}
