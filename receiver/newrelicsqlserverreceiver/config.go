// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config represents the receiver config settings within the collector's config.yaml
// Based on nri-mssql ArgumentList structure for compatibility
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Connection configuration
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Hostname string `mapstructure:"hostname"`
	Port     string `mapstructure:"port"`
	Instance string `mapstructure:"instance"`

	// Azure AD authentication
	ClientID     string `mapstructure:"client_id"`
	TenantID     string `mapstructure:"tenant_id"`
	ClientSecret string `mapstructure:"client_secret"`

	// SSL configuration
	EnableSSL              bool   `mapstructure:"enable_ssl"`
	TrustServerCertificate bool   `mapstructure:"trust_server_certificate"`
	CertificateLocation    string `mapstructure:"certificate_location"`

	// Performance and feature toggles
	EnableDatabaseSampleMetrics  bool `mapstructure:"enable_database_sample_metrics"`
	EnableBufferMetrics          bool `mapstructure:"enable_buffer_metrics"`
	EnableDatabaseReserveMetrics bool `mapstructure:"enable_database_reserve_metrics"`
	EnableDiskMetricsInBytes     bool `mapstructure:"enable_disk_metrics_in_bytes"`
	EnableIOMetrics              bool `mapstructure:"enable_io_metrics"`
	EnableLogGrowthMetrics       bool `mapstructure:"enable_log_growth_metrics"`
	EnablePageFileMetrics        bool `mapstructure:"enable_page_file_metrics"`
	EnablePageFileTotalMetrics   bool `mapstructure:"enable_page_file_total_metrics"`
	EnableMemoryMetrics          bool `mapstructure:"enable_memory_metrics"`
	EnableMemoryTotalMetrics     bool `mapstructure:"enable_memory_total_metrics"`
	EnableMemoryAvailableMetrics bool `mapstructure:"enable_memory_available_metrics"`
	EnableMemoryUtilizationMetrics bool `mapstructure:"enable_memory_utilization_metrics"`

	// Concurrency and timeouts
	MaxConcurrentWorkers int           `mapstructure:"max_concurrent_workers"`
	Timeout              time.Duration `mapstructure:"timeout"`

	// Custom queries
	CustomMetricsQuery  string `mapstructure:"custom_metrics_query"`
	CustomMetricsConfig string `mapstructure:"custom_metrics_config"`

	// Additional connection parameters
	ExtraConnectionURLArgs string `mapstructure:"extra_connection_url_args"`

	// Query monitoring configuration
	EnableQueryMonitoring                bool `mapstructure:"enable_query_monitoring"`
	QueryMonitoringResponseTimeThreshold int  `mapstructure:"query_monitoring_response_time_threshold"`
	QueryMonitoringCountThreshold        int  `mapstructure:"query_monitoring_count_threshold"`
	QueryMonitoringFetchInterval         int  `mapstructure:"query_monitoring_fetch_interval"`
}

// DefaultConfig returns a Config struct with default values
func DefaultConfig() component.Config {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),

		// Default connection settings
		Hostname: "127.0.0.1",
		Port:     "1433",

		// Default feature toggles (matching nri-mssql defaults)
		EnableDatabaseSampleMetrics:  false, // Master toggle - when true, enables all database metrics
		EnableBufferMetrics:          true,
		EnableDatabaseReserveMetrics: true,
		EnableDiskMetricsInBytes:     true,
		EnableIOMetrics:              true,
		EnableLogGrowthMetrics:       true,
		EnablePageFileMetrics:        true,
		EnablePageFileTotalMetrics:   true,
		EnableMemoryMetrics:          true,
		EnableMemoryTotalMetrics:     true,
		EnableMemoryAvailableMetrics: true,
		EnableMemoryUtilizationMetrics: true,

		// Default concurrency and timeout
		MaxConcurrentWorkers: 10,
		Timeout:              30 * time.Second,

		// Default SSL settings
		EnableSSL:              false,
		TrustServerCertificate: false,

		// Default query monitoring settings
		EnableQueryMonitoring:                false,
		QueryMonitoringResponseTimeThreshold: 1,
		QueryMonitoringCountThreshold:        20,
		QueryMonitoringFetchInterval:         15,
	}

	// Set default collection interval to 15 seconds
	cfg.ControllerConfig.CollectionInterval = 15 * time.Second

	return cfg
}

// Validate validates the configuration and sets defaults where needed
// Based on nri-mssql ArgumentList.Validate() method
func (cfg *Config) Validate() error {
	if cfg.Hostname == "" {
		return errors.New("hostname cannot be empty")
	}

	if cfg.Port != "" && cfg.Instance != "" {
		return errors.New("specify either port or instance but not both")
	} else if cfg.Port == "" && cfg.Instance == "" {
		// Default to port 1433 if neither is specified (matching nri-mssql behavior)
		cfg.Port = "1433"
	}

	if cfg.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}

	if cfg.MaxConcurrentWorkers <= 0 {
		return errors.New("max_concurrent_workers must be positive")
	}

	if cfg.EnableQueryMonitoring {
		if cfg.QueryMonitoringResponseTimeThreshold <= 0 {
			return errors.New("query_monitoring_response_time_threshold must be positive when query monitoring is enabled")
		}
		if cfg.QueryMonitoringCountThreshold <= 0 {
			return errors.New("query_monitoring_count_threshold must be positive when query monitoring is enabled")
		}
	}

	if cfg.EnableSSL && (!cfg.TrustServerCertificate && cfg.CertificateLocation == "") {
		return errors.New("must specify a certificate file when using SSL and not trusting server certificate")
	}

	if len(cfg.CustomMetricsConfig) > 0 {
		if len(cfg.CustomMetricsQuery) > 0 {
			return errors.New("cannot specify both custom_metrics_query and custom_metrics_config")
		}
		if _, err := os.Stat(cfg.CustomMetricsConfig); err != nil {
			return fmt.Errorf("custom_metrics_config file error: %w", err)
		}
	}

	return nil
}

// Unmarshal implements the confmap.Unmarshaler interface
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(cfg); err != nil {
		return err
	}
	return cfg.Validate()
}

// GetMaxConcurrentWorkers returns the configured max concurrent workers with fallback
// Based on nri-mssql ArgumentList.GetMaxConcurrentWorkers() method
func (cfg *Config) GetMaxConcurrentWorkers() int {
	if cfg.MaxConcurrentWorkers <= 0 {
		return 10 // DefaultMaxConcurrentWorkers from nri-mssql
	}
	return cfg.MaxConcurrentWorkers
}

// IsAzureADAuth checks if Azure AD Service Principal authentication is configured
func (cfg *Config) IsAzureADAuth() bool {
	return cfg.ClientID != "" && cfg.TenantID != "" && cfg.ClientSecret != ""
}

// CreateConnectionURL creates a connection string for SQL Server authentication
// Based on nri-mssql connection.CreateConnectionURL() method
func (cfg *Config) CreateConnectionURL(dbName string) string {
	connectionURL := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   cfg.Hostname,
	}

	// If port is present use port, if not use instance
	if cfg.Port != "" {
		connectionURL.Host = fmt.Sprintf("%s:%s", connectionURL.Host, cfg.Port)
	} else {
		connectionURL.Path = cfg.Instance
	}

	// Format query parameters
	query := url.Values{}
	query.Add("dial timeout", fmt.Sprintf("%.0f", cfg.Timeout.Seconds()))
	query.Add("connection timeout", fmt.Sprintf("%.0f", cfg.Timeout.Seconds()))

	if dbName != "" {
		query.Add("database", dbName)
	}

	if cfg.ExtraConnectionURLArgs != "" {
		extraArgsMap, err := url.ParseQuery(cfg.ExtraConnectionURLArgs)
		if err == nil {
			for k, v := range extraArgsMap {
				query.Add(k, v[0])
			}
		}
	}

	if cfg.EnableSSL {
		query.Add("encrypt", "true")
		query.Add("TrustServerCertificate", strconv.FormatBool(cfg.TrustServerCertificate))
		if !cfg.TrustServerCertificate && cfg.CertificateLocation != "" {
			query.Add("certificate", cfg.CertificateLocation)
		}
	}

	connectionURL.RawQuery = query.Encode()
	return connectionURL.String()
}

// CreateAzureADConnectionURL creates a connection string for Azure AD authentication
// Based on nri-mssql connection.CreateAzureADConnectionURL() method
func (cfg *Config) CreateAzureADConnectionURL(dbName string) string {
	connectionString := fmt.Sprintf(
		"server=%s;port=%s;fedauth=ActiveDirectoryServicePrincipal;applicationclientid=%s;clientsecret=%s;database=%s",
		cfg.Hostname,
		cfg.Port,
		cfg.ClientID,     // Client ID
		cfg.ClientSecret, // Client Secret
		dbName,           // Database
	)

	if cfg.ExtraConnectionURLArgs != "" {
		extraArgsMap, err := url.ParseQuery(cfg.ExtraConnectionURLArgs)
		if err == nil {
			for k, v := range extraArgsMap {
				connectionString += fmt.Sprintf(";%s=%s", k, v[0])
			}
		}
	}

	if cfg.EnableSSL {
		connectionString += ";encrypt=true"
		if cfg.TrustServerCertificate {
			connectionString += ";TrustServerCertificate=true"
		} else {
			connectionString += ";TrustServerCertificate=false"
			if cfg.CertificateLocation != "" {
				connectionString += fmt.Sprintf(";certificate=%s", cfg.CertificateLocation)
			}
		}
	}

	return connectionString
}

// Helper methods to check if metrics are enabled (either individually or via master toggle)

// IsBufferMetricsEnabled checks if buffer metrics should be collected
func (cfg *Config) IsBufferMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableBufferMetrics
}

// IsDatabaseReserveMetricsEnabled checks if database reserve metrics should be collected
func (cfg *Config) IsDatabaseReserveMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableDatabaseReserveMetrics
}

// IsDiskMetricsInBytesEnabled checks if disk metrics in bytes should be collected
func (cfg *Config) IsDiskMetricsInBytesEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableDiskMetricsInBytes
}

// IsIOMetricsEnabled checks if IO metrics should be collected
func (cfg *Config) IsIOMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableIOMetrics
}

// IsLogGrowthMetricsEnabled checks if log growth metrics should be collected
func (cfg *Config) IsLogGrowthMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableLogGrowthMetrics
}

// IsPageFileMetricsEnabled checks if page file metrics should be collected
func (cfg *Config) IsPageFileMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnablePageFileMetrics
}

// IsPageFileTotalMetricsEnabled checks if page file total metrics should be collected
func (cfg *Config) IsPageFileTotalMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnablePageFileTotalMetrics
}

// IsMemoryMetricsEnabled checks if memory metrics should be collected
func (cfg *Config) IsMemoryMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableMemoryMetrics
}

// IsMemoryTotalMetricsEnabled checks if memory total metrics should be collected
func (cfg *Config) IsMemoryTotalMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableMemoryTotalMetrics
}

// IsMemoryAvailableMetricsEnabled checks if memory available metrics should be collected
func (cfg *Config) IsMemoryAvailableMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableMemoryAvailableMetrics
}

// IsMemoryUtilizationMetricsEnabled checks if memory utilization metrics should be collected
func (cfg *Config) IsMemoryUtilizationMetricsEnabled() bool {
	return cfg.EnableDatabaseSampleMetrics || cfg.EnableMemoryUtilizationMetrics
}
