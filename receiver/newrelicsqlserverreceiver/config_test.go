// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_minimal_config",
			config: &Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Hostname:             "localhost",
				Port:                 "1433",
				Username:             "test_user",
				Password:             "test_password",
				Timeout:              30 * time.Second,
				MaxConcurrentWorkers: 5,
			},
			wantErr: false,
		},
		{
			name: "valid_full_config",
			config: &Config{
				ControllerConfig:                     scraperhelper.NewDefaultControllerConfig(),
				Hostname:                             "sql-server.example.com",
				Port:                                 "1433",
				Username:                             "monitoring_user",
				Password:                             "secure_password",
				ClientID:                             "azure-client-id",
				TenantID:                             "azure-tenant-id",
				ClientSecret:                         "azure-client-secret",
				EnableSSL:                            true,
				TrustServerCertificate:               true,
				CertificateLocation:                  "/path/to/cert.pem",
				EnableBufferMetrics:                  true,
				EnableDatabaseReserveMetrics:         true,
				EnableDiskMetricsInBytes:             true,
				MaxConcurrentWorkers:                 10,
				Timeout:                              45 * time.Second,
				CustomMetricsQuery:                   "SELECT * FROM sys.dm_os_performance_counters",
				ExtraConnectionURLArgs:               "encrypt=true;trustServerCertificate=false",
				EnableQueryMonitoring:                true,
				QueryMonitoringResponseTimeThreshold: 5,
				QueryMonitoringCountThreshold:        50,
				QueryMonitoringFetchInterval:         30,
			},
			wantErr: false,
		},
		{
			name: "invalid_empty_hostname",
			config: &Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Hostname:             "",
				Port:                 "1433",
				Username:             "test_user",
				Password:             "test_password",
				Timeout:              30 * time.Second,
				MaxConcurrentWorkers: 5,
			},
			wantErr: true,
			errMsg:  "hostname cannot be empty",
		},
		{
			name: "invalid_both_port_and_instance",
			config: &Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Hostname:             "localhost",
				Port:                 "1433",
				Instance:             "MSSQLSERVER",
				Username:             "test_user",
				Password:             "test_password",
				Timeout:              30 * time.Second,
				MaxConcurrentWorkers: 5,
			},
			wantErr: true,
			errMsg:  "specify either port or instance but not both",
		},
		{
			name: "invalid_negative_timeout",
			config: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Hostname:         "localhost",
				Port:             "1433",
				Username:         "test_user",
				Password:         "test_password",
				Timeout:          -5 * time.Second,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid_zero_timeout",
			config: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Hostname:         "localhost",
				Port:             "1433",
				Username:         "test_user",
				Password:         "test_password",
				Timeout:          0,
			},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
		{
			name: "invalid_negative_max_workers",
			config: &Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Hostname:             "localhost",
				Port:                 "1433",
				Username:             "test_user",
				Password:             "test_password",
				Timeout:              30 * time.Second,
				MaxConcurrentWorkers: -1,
			},
			wantErr: true,
			errMsg:  "max_concurrent_workers must be positive",
		},
		{
			name: "invalid_zero_max_workers",
			config: &Config{
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Hostname:             "localhost",
				Port:                 "1433",
				Username:             "test_user",
				Password:             "test_password",
				Timeout:              30 * time.Second,
				MaxConcurrentWorkers: 0,
			},
			wantErr: true,
			errMsg:  "max_concurrent_workers must be positive",
		},
		{
			name: "invalid_negative_query_threshold",
			config: &Config{
				ControllerConfig:                     scraperhelper.NewDefaultControllerConfig(),
				Hostname:                             "localhost",
				Port:                                 "1433",
				Username:                             "test_user",
				Password:                             "test_password",
				Timeout:                              30 * time.Second,
				MaxConcurrentWorkers:                 5,
				EnableQueryMonitoring:                true,
				QueryMonitoringResponseTimeThreshold: -1,
			},
			wantErr: true,
			errMsg:  "query_monitoring_response_time_threshold must be positive when query monitoring is enabled",
		},
		{
			name: "invalid_negative_count_threshold",
			config: &Config{
				ControllerConfig:                     scraperhelper.NewDefaultControllerConfig(),
				Hostname:                             "localhost",
				Port:                                 "1433",
				Username:                             "test_user",
				Password:                             "test_password",
				Timeout:                              30 * time.Second,
				MaxConcurrentWorkers:                 5,
				EnableQueryMonitoring:                true,
				QueryMonitoringResponseTimeThreshold: 5,
				QueryMonitoringCountThreshold:        1,
			},
			wantErr: true,
			errMsg:  "query_monitoring_count_threshold must be positive when query monitoring is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig().(*Config)

	// Test all default values
	assert.Equal(t, "127.0.0.1", cfg.Hostname)
	assert.Equal(t, "1433", cfg.Port)
	assert.Equal(t, "", cfg.Username)
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, "", cfg.Instance)

	// Azure AD defaults
	assert.Equal(t, "", cfg.ClientID)
	assert.Equal(t, "", cfg.TenantID)
	assert.Equal(t, "", cfg.ClientSecret)

	// SSL defaults
	assert.False(t, cfg.EnableSSL)
	assert.False(t, cfg.TrustServerCertificate)
	assert.Equal(t, "", cfg.CertificateLocation)

	// Feature defaults
	assert.True(t, cfg.EnableBufferMetrics)
	assert.True(t, cfg.EnableDatabaseReserveMetrics)
	assert.True(t, cfg.EnableDiskMetricsInBytes)

	// Performance defaults
	assert.Equal(t, 10, cfg.MaxConcurrentWorkers)
	assert.Equal(t, 30*time.Second, cfg.Timeout)

	// Query monitoring defaults
	assert.False(t, cfg.EnableQueryMonitoring)
	assert.Equal(t, 1, cfg.QueryMonitoringResponseTimeThreshold)
	assert.Equal(t, 20, cfg.QueryMonitoringCountThreshold)
	assert.Equal(t, 15, cfg.QueryMonitoringFetchInterval)

	// Collection interval default
	assert.Equal(t, 15*time.Second, cfg.ControllerConfig.CollectionInterval)
}

func TestConfigCreation(t *testing.T) {
	// Test basic config creation and field assignment
	config := &Config{
		Hostname:                             "test-server",
		Port:                                 "1434",
		Instance:                             "NAMED_INSTANCE",
		Username:                             "monitoring_user",
		Password:                             "secure_password",
		ClientID:                             "azure-client-id",
		TenantID:                             "azure-tenant-id",
		ClientSecret:                         "azure-client-secret",
		EnableSSL:                            true,
		TrustServerCertificate:               true,
		CertificateLocation:                  "/path/to/cert.pem",
		EnableBufferMetrics:                  false,
		EnableDatabaseReserveMetrics:         false,
		EnableDiskMetricsInBytes:             false,
		MaxConcurrentWorkers:                 15,
		Timeout:                              60 * time.Second,
		CustomMetricsQuery:                   "SELECT 1",
		CustomMetricsConfig:                  "/path/to/config.yml",
		ExtraConnectionURLArgs:               "encrypt=true",
		EnableQueryMonitoring:                true,
		QueryMonitoringResponseTimeThreshold: 10,
		QueryMonitoringCountThreshold:        100,
		QueryMonitoringFetchInterval:         60,
	}

	// Verify all fields are set correctly
	assert.Equal(t, "test-server", config.Hostname)
	assert.Equal(t, "1434", config.Port)
	assert.Equal(t, "NAMED_INSTANCE", config.Instance)
	assert.Equal(t, "monitoring_user", config.Username)
	assert.Equal(t, "secure_password", config.Password)
	assert.Equal(t, "azure-client-id", config.ClientID)
	assert.Equal(t, "azure-tenant-id", config.TenantID)
	assert.Equal(t, "azure-client-secret", config.ClientSecret)
	assert.True(t, config.EnableSSL)
	assert.True(t, config.TrustServerCertificate)
	assert.Equal(t, "/path/to/cert.pem", config.CertificateLocation)
	assert.False(t, config.EnableBufferMetrics)
	assert.False(t, config.EnableDatabaseReserveMetrics)
	assert.False(t, config.EnableDiskMetricsInBytes)
	assert.Equal(t, 15, config.MaxConcurrentWorkers)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, "SELECT 1", config.CustomMetricsQuery)
	assert.Equal(t, "/path/to/config.yml", config.CustomMetricsConfig)
	assert.Equal(t, "encrypt=true", config.ExtraConnectionURLArgs)
	assert.True(t, config.EnableQueryMonitoring)
	assert.Equal(t, 10, config.QueryMonitoringResponseTimeThreshold)
	assert.Equal(t, 100, config.QueryMonitoringCountThreshold)
	assert.Equal(t, 60, config.QueryMonitoringFetchInterval)
}
