// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/open-telemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestValidateInvalidConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "Empty endpoint",
			config: &Config{
				Endpoint:         "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "Missing port in endpoint",
			config: &Config{
				Endpoint:         "localhost",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Invalid endpoint format",
			config: &Config{
				Endpoint:         "x;;ef;s;d:::ss:23423423423423423",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Missing host in endpoint",
			config: &Config{
				Endpoint:         ":3001",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Negative port",
			config: &Config{
				Endpoint:         "localhost:-2",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "Bad port",
			config: &Config{
				Endpoint:         "localhost:9999999999999999999",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "Empty username",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "",
				Password:         "secret",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyUsername,
		},
		{
			name: "Empty password",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "ro_user",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyPassword,
		},
		{
			name: "Empty service",
			config: &Config{
				Endpoint:         "localhost:3000",
				Password:         "password",
				Username:         "ro_user",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyService,
		},
		{
			name: "Invalid data source",
			config: &Config{
				DataSource:       "%%%",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadDataSource,
		},
		{
			name: "Invalid auth_type",
			config: &Config{
				Endpoint:         "localhost:3000",
				Service:          "XE",
				AuthType:         "oauth",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errInvalidAuthType,
		},
		{
			name: "Kerberos auth_type without kerberos block",
			config: &Config{
				Endpoint:         "localhost:3000",
				Service:          "XE",
				AuthType:         AuthTypeKerberos,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errMissingKerberosBlock,
		},
		{
			name: "Kerberos block set without kerberos auth_type",
			config: &Config{
				Endpoint:         "localhost:3000",
				Service:          "XE",
				Username:         "otel",
				Password:         "secret",
				Kerberos:         &KerberosConfig{CredentialType: KerberosCredentialKeytab},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errUnexpectedKerberosBlock,
		},
		{
			name: "Kerberos missing realm",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosRealm,
		},
		{
			name: "Kerberos missing principal",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosPrincipal,
		},
		{
			name: "Kerberos principal includes realm",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel@EXAMPLE.COM",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errKerberosPrincipalRealm,
		},
		{
			name: "Kerberos missing config_file",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosConfigFile,
		},
		{
			name: "Kerberos keytab credential missing keytab_file",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosKeytab,
		},
		{
			name: "Kerberos ccache credential missing credential_cache",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					// realm and principal intentionally omitted: the ccache
					// credential type reads them from the cache file and must
					// not require them.
					CredentialType: KerberosCredentialCache,
					ConfigFile:     "/etc/krb5.conf",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosCredCache,
		},
		{
			name: "Kerberos password credential missing password",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialPassword,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyKerberosPassword,
		},
		{
			name: "Kerberos invalid credential_type",
			config: &Config{
				Endpoint: "localhost:3000",
				Service:  "XE",
				AuthType: AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: "smartcard",
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errInvalidCredentialType,
		},
		{
			name: "Kerberos data source with username",
			config: &Config{
				DataSource: "oracle://otel@host:1521/XE",
				AuthType:   AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errKerberosDataSourceCreds,
		},
		{
			name: "Kerberos data source with username and password",
			config: &Config{
				DataSource: "oracle://otel:secret@host:1521/XE",
				AuthType:   AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errKerberosDataSourceCreds,
		},
		{
			name: "Kerberos data source with AUTH TYPE",
			config: &Config{
				DataSource: "oracle://host:1521/XE?AUTH+TYPE=TCPS",
				AuthType:   AuthTypeKerberos,
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errKerberosDataSourceCreds,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestValidateValidKerberosConfigs(t *testing.T) {
	base := func(k *KerberosConfig) *Config {
		return &Config{
			Endpoint:         "localhost:3000",
			Service:          "XE",
			AuthType:         AuthTypeKerberos,
			Kerberos:         k,
			ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			TopQueryCollection: TopQueryCollection{
				MaxQuerySampleCount: 1000,
				TopQueryCount:       200,
			},
		}
	}

	testCases := []struct {
		name   string
		config *Config
	}{
		{
			name: "keytab",
			config: base(&KerberosConfig{
				CredentialType: KerberosCredentialKeytab,
				Realm:          "EXAMPLE.COM",
				Principal:      "otel",
				ConfigFile:     "/etc/krb5.conf",
				KeytabFile:     "/etc/otel.keytab",
			}),
		},
		{
			// realm and principal are omitted on purpose: the ccache credential
			// type derives them from the cache file, so a config without them is
			// valid.
			name: "ccache",
			config: base(&KerberosConfig{
				CredentialType:  KerberosCredentialCache,
				ConfigFile:      "/etc/krb5.conf",
				CredentialCache: "/tmp/krb5cc_1000",
			}),
		},
		{
			name: "password",
			config: base(&KerberosConfig{
				CredentialType: KerberosCredentialPassword,
				Realm:          "EXAMPLE.COM",
				Principal:      "otel",
				ConfigFile:     "/etc/krb5.conf",
				Password:       "s3cret",
			}),
		},
		{
			name: "data source without credentials or auth type",
			config: &Config{
				DataSource:       "oracle://host:1521/XE?SSL=true",
				AuthType:         AuthTypeKerberos,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				TopQueryCollection: TopQueryCollection{
					MaxQuerySampleCount: 1000,
					TopQueryCount:       200,
				},
				Kerberos: &KerberosConfig{
					CredentialType: KerberosCredentialKeytab,
					Realm:          "EXAMPLE.COM",
					Principal:      "otel",
					ConfigFile:     "/etc/krb5.conf",
					KeytabFile:     "/etc/otel.keytab",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.config.Validate())
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.ControllerConfig.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub("oracledb")
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)

	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	assert.Equal(t, "oracle://otel:password@localhost:51521/XE", cfg.DataSource)
	assert.Equal(t, "otel", cfg.Username)
	assert.Equal(t, "password", cfg.Password)
	assert.Equal(t, "localhost:51521", cfg.Endpoint)
	assert.Equal(t, "XE", cfg.Service)
	settings := cfg.MetricsBuilderConfig.Metrics
	assert.False(t, settings.OracledbTablespaceSizeUsage.Enabled)
	assert.False(t, settings.OracledbExchangeDeadlocks.Enabled)
}
