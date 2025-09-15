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
	EnableBufferMetrics          bool `mapstructure:"enable_buffer_metrics"`
	EnableDatabaseReserveMetrics bool `mapstructure:"enable_database_reserve_metrics"`
	EnableDiskMetricsInBytes     bool `mapstructure:"enable_disk_metrics_in_bytes"`

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
		EnableBufferMetrics:          true,
		EnableDatabaseReserveMetrics: true,
		EnableDiskMetricsInBytes:     true,

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
		return errors.New("invalid configuration: must specify a hostname")
	}

	if cfg.Port != "" && cfg.Instance != "" {
		return errors.New("invalid configuration: specify either port or instance but not both")
	} else if cfg.Port == "" && cfg.Instance == "" {
		// Default to port 1433 if neither is specified (matching nri-mssql behavior)
		cfg.Port = "1433"
	}

	if cfg.EnableSSL && (!cfg.TrustServerCertificate && cfg.CertificateLocation == "") {
		return errors.New("invalid configuration: must specify a certificate file when using SSL and not trusting server certificate")
	}

	if len(cfg.CustomMetricsConfig) > 0 {
		if len(cfg.CustomMetricsQuery) > 0 {
			return errors.New("cannot specify both custom_metrics_query and custom_metrics_config")
		}
		if _, err := os.Stat(cfg.CustomMetricsConfig); err != nil {
			return fmt.Errorf("custom_metrics_config file error: %w", err)
		}
	}

	if cfg.MaxConcurrentWorkers <= 0 {
		cfg.MaxConcurrentWorkers = 10 // Default from nri-mssql
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
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
		"server=%s;port=%s;database=%s;user id=%s@%s;password=%s;fedauth=ActiveDirectoryServicePrincipal;dial timeout=%.0f;connection timeout=%.0f",
		cfg.Hostname,
		cfg.Port,
		dbName,
		cfg.ClientID,     // Client ID
		cfg.TenantID,     // Tenant ID
		cfg.ClientSecret, // Client Secret
		cfg.Timeout.Seconds(),
		cfg.Timeout.Seconds(),
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
