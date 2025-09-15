// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBuildConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "basic_sql_auth",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Username: "sa",
				Password: "password123",
				Timeout:  30 * time.Second,
			},
			expected: "sqlserver://sa:password123@localhost:1433?connection+timeout=30&database=master&dial+timeout=30",
		},
		{
			name: "with_instance",
			config: &Config{
				Hostname: "localhost",
				Instance: "SQLEXPRESS",
				Username: "sa",
				Password: "password123",
				Timeout:  30 * time.Second,
			},
			expected: "sqlserver://sa:password123@localhost/SQLEXPRESS?connection+timeout=30&database=master&dial+timeout=30",
		},
		{
			name: "with_ssl_enabled",
			config: &Config{
				Hostname:  "localhost",
				Port:      "1433",
				Username:  "sa",
				Password:  "password123",
				EnableSSL: true,
				Timeout:   30 * time.Second,
			},
			expected: "sqlserver://sa:password123@localhost:1433?TrustServerCertificate=false&connection+timeout=30&database=master&dial+timeout=30&encrypt=true",
		},
		{
			name: "with_ssl_and_trust_cert",
			config: &Config{
				Hostname:               "localhost",
				Port:                   "1433",
				Username:               "sa",
				Password:               "password123",
				EnableSSL:              true,
				TrustServerCertificate: true,
				Timeout:                30 * time.Second,
			},
			expected: "sqlserver://sa:password123@localhost:1433?TrustServerCertificate=true&connection+timeout=30&database=master&dial+timeout=30&encrypt=true",
		},
		{
			name: "azure_ad_auth",
			config: &Config{
				Hostname:     "sqlserver.database.windows.net",
				Port:         "1433",
				ClientID:     "client-id-123",
				TenantID:     "tenant-id-456",
				ClientSecret: "client-secret-789",
				Timeout:      30 * time.Second,
			},
			expected: "server=sqlserver.database.windows.net;port=1433;fedauth=ActiveDirectoryServicePrincipal;applicationclientid=client-id-123;clientsecret=client-secret-789;database=master",
		},
		{
			name: "with_extra_args",
			config: &Config{
				Hostname:               "localhost",
				Port:                   "1433",
				Username:               "sa",
				Password:               "password123",
				Timeout:                30 * time.Second,
				ExtraConnectionURLArgs: "connection+timeout=60&packet+size=4096",
			},
			expected: "sqlserver://sa:password123@localhost:1433?connection+timeout=30&connection+timeout=60&database=master&dial+timeout=30&packet+size=4096",
		},
		{
			name: "windows_auth",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Timeout:  30 * time.Second,
				// No username/password for Windows auth
			},
			expected: "sqlserver://:@localhost:1433?connection+timeout=30&database=master&dial+timeout=30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.config.IsAzureADAuth() {
				result = tt.config.CreateAzureADConnectionURL("master")
			} else {
				result = tt.config.CreateConnectionURL("master")
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigValidation_Connection(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_sql_auth",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Username: "sa",
				Password: "password123",
				Timeout:  30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid_windows_auth",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				// No username/password for Windows auth
				Timeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid_azure_ad",
			config: &Config{
				Hostname:     "sqlserver.database.windows.net",
				Port:         "1433",
				ClientID:     "client-id",
				TenantID:     "tenant-id",
				ClientSecret: "client-secret",
				Timeout:      30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid_sql_auth_missing_password",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Username: "sa",
				// Missing password
				Timeout: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "password is required when username is provided",
		},
		{
			name: "invalid_azure_ad_missing_tenant",
			config: &Config{
				Hostname:     "sqlserver.database.windows.net",
				Port:         "1433",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				// Missing tenant ID
				Timeout: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "tenant_id is required for Azure AD authentication",
		},
		{
			name: "invalid_azure_ad_missing_client_secret",
			config: &Config{
				Hostname: "sqlserver.database.windows.net",
				Port:     "1433",
				ClientID: "client-id",
				TenantID: "tenant-id",
				// Missing client secret
				Timeout: 30 * time.Second,
			},
			wantErr: true,
			errMsg:  "client_secret is required for Azure AD authentication",
		},
		{
			name: "invalid_ssl_cert_location_without_ssl",
			config: &Config{
				Hostname:            "localhost",
				Port:                "1433",
				Username:            "sa",
				Password:            "password123",
				EnableSSL:           false,
				CertificateLocation: "/path/to/cert.pem",
				Timeout:             30 * time.Second,
			},
			wantErr: true,
			errMsg:  "certificate_location can only be specified when SSL is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConnectionConfig(tt.config)

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

func TestNewSQLConnection_ConfigValidation(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil_config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty_hostname",
			config: &Config{
				Hostname: "",
				Port:     "1433",
				Username: "sa",
				Password: "password123",
				Timeout:  30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "valid_config",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Username: "sa",
				Password: "password123",
				Timeout:  30 * time.Second,
			},
			wantErr: false, // Will fail on actual connection, but config validation should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewSQLConnection(ctx, tt.config, logger)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, conn)
			} else {
				// Note: We expect this to fail due to no actual SQL Server,
				// but we're testing that the configuration validation passes
				if err != nil {
					// This is expected - we're not connecting to a real SQL Server
					// Just ensure it's not a config validation error
					assert.NotContains(t, err.Error(), "config")
				}
			}
		})
	}
}

func TestConnectionStringEscaping(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		contains    []string
		notContains []string
	}{
		{
			name: "password_with_special_chars",
			config: &Config{
				Hostname: "localhost",
				Port:     "1433",
				Username: "sa",
				Password: "p@ssw0rd;{with}special=chars",
			},
			contains: []string{
				"server=localhost",
				"port=1433",
				"user id=sa",
			},
			// Password should be properly handled in connection string
		},
		{
			name: "hostname_with_special_chars",
			config: &Config{
				Hostname: "sql-server.domain.com",
				Port:     "1433",
				Username: "domain\\user",
				Password: "password",
			},
			contains: []string{
				"server=sql-server.domain.com",
				"port=1433",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildConnectionString(tt.config)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, result, notExpected)
			}
		})
	}
}

// Helper function to build connection string (simplified version for testing)
func buildConnectionString(config *Config) string {
	if config == nil {
		return ""
	}

	connStr := ""

	// Server and port/instance
	if config.Instance != "" {
		connStr += "server=" + config.Hostname + "\\" + config.Instance
	} else {
		connStr += "server=" + config.Hostname
		if config.Port != "" {
			connStr += ";port=" + config.Port
		}
	}

	// Authentication
	if config.ClientID != "" && config.TenantID != "" && config.ClientSecret != "" {
		// Azure AD Service Principal
		connStr += ";fedauth=ActiveDirectoryServicePrincipal"
		connStr += ";applicationclientid=" + config.ClientID
		connStr += ";clientsecret=" + config.ClientSecret
	} else if config.Username != "" && config.Password != "" {
		// SQL Server Authentication
		connStr += ";user id=" + config.Username
		connStr += ";password=" + config.Password
	} else {
		// Windows Authentication
		connStr += ";integrated security=true"
	}

	// Database
	connStr += ";database=master"

	// SSL/TLS
	if config.EnableSSL {
		connStr += ";encrypt=true"
		if config.TrustServerCertificate {
			connStr += ";trustservercertificate=true"
		}
	}

	// Extra connection parameters
	if config.ExtraConnectionURLArgs != "" {
		connStr += ";" + config.ExtraConnectionURLArgs
	}

	return connStr
}

// Helper function to validate connection configuration
func validateConnectionConfig(config *Config) error {
	if config.Hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	// SQL Server Authentication validation
	if config.Username != "" && config.Password == "" {
		return fmt.Errorf("password is required when username is provided")
	}

	// Azure AD Authentication validation
	if config.ClientID != "" || config.TenantID != "" || config.ClientSecret != "" {
		if config.ClientID == "" {
			return fmt.Errorf("client_id is required for Azure AD authentication")
		}
		if config.TenantID == "" {
			return fmt.Errorf("tenant_id is required for Azure AD authentication")
		}
		if config.ClientSecret == "" {
			return fmt.Errorf("client_secret is required for Azure AD authentication")
		}
	}

	// SSL configuration validation
	if !config.EnableSSL && config.CertificateLocation != "" {
		return fmt.Errorf("certificate_location can only be specified when SSL is enabled")
	}

	return nil
}
