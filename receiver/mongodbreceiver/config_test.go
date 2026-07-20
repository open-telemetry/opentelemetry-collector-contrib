// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		endpoints []string
		desc      string
		username  string
		password  string
		scheme    string
		expected  error
	}{
		{
			desc:      "no username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "",
			expected:  nil,
		},
		{
			desc:      "no username, with password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "pass",
			expected:  errors.New("password provided without user"),
		},
		{
			desc:      "with username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "",
			expected:  errors.New("username provided without password"),
		},
		{
			desc:      "with username and password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "pass",
			expected:  nil,
		},
		{
			desc:     "no hosts",
			username: "user",
			password: "pass",
			expected: errors.New("no hosts were specified in the config"),
		},
		{
			desc:      "valid hostname",
			endpoints: []string{"localhost"},
			expected:  nil,
		},
		{
			desc:      "empty host",
			username:  "user",
			endpoints: []string{""},
			expected:  errors.New("no endpoint specified for one of the hosts"),
		},
		{
			desc:      "scheme mongodb is valid",
			endpoints: []string{"localhost:27017"},
			scheme:    "mongodb",
			expected:  nil,
		},
		{
			desc:      "scheme mongodb+srv with one host is valid",
			endpoints: []string{"cluster0.example.mongodb.net"},
			scheme:    "mongodb+srv",
			expected:  nil,
		},
		{
			desc:      "scheme mongodb+srv with multiple hosts is invalid",
			endpoints: []string{"host1.example.net", "host2.example.net"},
			scheme:    "mongodb+srv",
			expected:  errors.New("mongodb+srv scheme requires exactly one host"),
		},
		{
			desc:      "invalid scheme",
			endpoints: []string{"localhost:27017"},
			scheme:    "invalid",
			expected:  errors.New("invalid scheme \"invalid\", must be \"mongodb\" or \"mongodb+srv\""),
		},
		{
			desc:      "empty scheme defaults to mongodb",
			endpoints: []string{"localhost:27017"},
			scheme:    "",
			expected:  nil,
		},
	}
	// max_rows_per_query=0 must be validated separately since the test loop
	// above always sets it to defaultMaxRowsPerQuery.
	t.Run("max_rows_per_query zero is invalid", func(t *testing.T) {
		cfg := &Config{
			Hosts:                 []confignet.TCPAddrConfig{{Endpoint: "localhost:27017"}},
			ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
			MetricsBuilderConfig:  metadata.NewDefaultMetricsBuilderConfig(),
			QuerySampleCollection: QuerySampleCollection{MaxRowsPerQuery: 0},
			TopQueryCollection:    defaultTopQueryCollection(),
		}
		err := xconfmap.Validate(cfg)
		require.ErrorContains(t, err, "query_sample_collection.max_rows_per_query must be greater than 0")
	})
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var hosts []confignet.TCPAddrConfig

			for _, ep := range tc.endpoints {
				hosts = append(hosts, confignet.TCPAddrConfig{
					Endpoint: ep,
				})
			}

			cfg := &Config{
				Username:              tc.username,
				Password:              configopaque.String(tc.password),
				Hosts:                 hosts,
				Scheme:                tc.scheme,
				ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig:  metadata.NewDefaultMetricsBuilderConfig(),
				QuerySampleCollection: QuerySampleCollection{MaxRowsPerQuery: defaultMaxRowsPerQuery},
				TopQueryCollection:    defaultTopQueryCollection(),
			}
			err := xconfmap.Validate(cfg)
			if tc.expected == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expected.Error())
			}
		})
	}
}

func TestBadTLSConfigs(t *testing.T) {
	testCases := []struct {
		desc        string
		tlsConfig   configtls.ClientConfig
		expectError bool
	}{
		{
			desc: "CA file not found",
			tlsConfig: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "not/a/real/file.pem",
				},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: true,
		},
		{
			desc: "no issues",
			tlsConfig: configtls.ClientConfig{
				Config:             configtls.Config{},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := &Config{
				Username: "otel",
				Password: "pword",
				Hosts: []confignet.TCPAddrConfig{
					{
						Endpoint: defaultEndpoint,
					},
				},
				ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
				ClientConfig:          tc.tlsConfig,
				MetricsBuilderConfig:  metadata.NewDefaultMetricsBuilderConfig(),
				QuerySampleCollection: QuerySampleCollection{MaxRowsPerQuery: defaultMaxRowsPerQuery},
				TopQueryCollection:    defaultTopQueryCollection(),
			}
			err := xconfmap.Validate(cfg)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		Username:   "uname",
		Password:   "password",
		Timeout:    2 * time.Minute,
		ReplicaSet: "rs-1",
	}

	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, clientOptions.Auth.Username, cfg.Username)
	require.Equal(t,
		clientOptions.ConnectTimeout.Milliseconds(),
		(2 * time.Minute).Milliseconds(),
	)
	require.Equal(t, "rs-1", *clientOptions.ReplicaSet)
}

func TestOptionsWithAuthMechanismAndSource(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		Username:      "uname",
		Password:      "password",
		AuthMechanism: "SCRAM-SHA-256",
		AuthSource:    "admin",
		AuthMechanismProperties: map[string]string{
			"SERVICE_NAME": "mongodb",
		},
		Timeout:    2 * time.Minute,
		ReplicaSet: "rs-1",
	}

	// Test primary connection options
	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, clientOptions.Auth.Username, cfg.Username)
	require.Equal(t, clientOptions.Auth.AuthMechanism, cfg.AuthMechanism)
	require.Equal(t, clientOptions.Auth.AuthSource, cfg.AuthSource)
	require.Equal(t, clientOptions.Auth.AuthMechanismProperties, cfg.AuthMechanismProperties)
	require.Equal(t,
		clientOptions.ConnectTimeout.Milliseconds(),
		(2 * time.Minute).Milliseconds(),
	)
	require.Equal(t, "rs-1", *clientOptions.ReplicaSet)

	// Test secondary connection options
	secondaryOptions := cfg.ClientOptions(true)
	require.Equal(t, secondaryOptions.Auth.Username, cfg.Username)
	require.Equal(t, secondaryOptions.Auth.AuthMechanism, cfg.AuthMechanism)
	require.Equal(t, secondaryOptions.Auth.AuthSource, cfg.AuthSource)
	require.Equal(t, secondaryOptions.Auth.AuthMechanismProperties, cfg.AuthMechanismProperties)
}

func TestOptionsWithAuthMechanismOnly(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		AuthMechanism: "MONGODB-X509",
		AuthSource:    "$external",
		Timeout:       2 * time.Minute,
	}

	// Test primary connection options
	clientOptions := cfg.ClientOptions(false)
	require.Empty(t, clientOptions.Auth.Username)
	require.Empty(t, clientOptions.Auth.Password)
	require.Equal(t, "MONGODB-X509", clientOptions.Auth.AuthMechanism)
	require.Equal(t, "$external", clientOptions.Auth.AuthSource)

	// Test secondary connection options
	secondaryOptions := cfg.ClientOptions(true)
	require.Empty(t, secondaryOptions.Auth.Username)
	require.Empty(t, secondaryOptions.Auth.Password)
	require.Equal(t, "MONGODB-X509", secondaryOptions.Auth.AuthMechanism)
	require.Equal(t, "$external", secondaryOptions.Auth.AuthSource)
}

func TestOptionsDefaultScheme(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: "localhost:27017",
			},
		},
	}

	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, []string{"localhost:27017"}, clientOptions.Hosts)
}

func TestOptionsSRVScheme(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: "cluster0.example.mongodb.net",
			},
		},
		Scheme: "mongodb+srv",
	}

	clientOptions := cfg.ClientOptions(false)
	require.NotNil(t, clientOptions)
	// mongodb+srv:// defers host resolution to connect time,
	// so Hosts is not populated at parse time
	require.Nil(t, clientOptions.Hosts)
}

func TestOptionsTLS(t *testing.T) {
	// loading valid ca file
	caFile := filepath.Join("testdata", "certs", "ca.crt")

	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		ClientConfig: configtls.ClientConfig{
			Insecure: false,
			Config: configtls.Config{
				CAFile: caFile,
			},
		},
	}
	opts := cfg.ClientOptions(false)
	require.NotNil(t, opts.TLSConfig)
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Hosts = []confignet.TCPAddrConfig{
		{
			Endpoint: defaultEndpoint,
		},
	}
	expected.Username = "otel"
	expected.Password = "${env:MONGO_PASSWORD}"
	expected.CollectionInterval = time.Minute
	expected.AuthMechanism = "SCRAM-SHA-256"
	expected.AuthSource = "admin"
	expected.AuthMechanismProperties = map[string]string{
		"SERVICE_NAME": "mongodb",
	}
	expected.QuerySampleCollection.MaxRowsPerQuery = 42

	require.Equal(t, expected, cfg)
}

func TestLoadConfigSRV(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "srv").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Hosts = []confignet.TCPAddrConfig{
		{
			Endpoint: "cluster0.example.mongodb.net",
		},
	}
	expected.Scheme = "mongodb+srv"
	expected.Username = "otel"
	expected.Password = "${env:MONGO_PASSWORD}"
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}

// defaultTopQueryCollection returns a fully-populated TopQueryCollection
// matching the factory defaults, so validation-focused unit tests can vary
// one field at a time.
func defaultTopQueryCollection() TopQueryCollection {
	return TopQueryCollection{
		CollectionInterval:     defaultTopQueryCollectionInterval,
		MaxQuerySampleCount:    defaultMaxQuerySampleCount,
		MaxExplainEachInterval: defaultMaxExplainEachInterval,
		TopQueryCount:          defaultTopQueryCount,
		QueryPlanCacheSize:     defaultQueryPlanCacheSize,
		QueryPlanCacheTTL:      defaultQueryPlanCacheTTL,
	}
}

func TestValidateTopQueryCollection(t *testing.T) {
	baseCfg := func() *Config {
		lbc := metadata.DefaultLogsBuilderConfig()
		lbc.Events.DbServerTopQuery.Enabled = true
		return &Config{
			Hosts:                 []confignet.TCPAddrConfig{{Endpoint: "localhost:27017"}},
			ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
			MetricsBuilderConfig:  metadata.NewDefaultMetricsBuilderConfig(),
			LogsBuilderConfig:     lbc,
			QuerySampleCollection: QuerySampleCollection{MaxRowsPerQuery: defaultMaxRowsPerQuery},
			TopQueryCollection:    defaultTopQueryCollection(),
		}
	}

	testCases := []struct {
		desc      string
		mutate    func(c *TopQueryCollection)
		errSubstr string // empty => expect no error
	}{
		{
			desc:   "defaults are valid",
			mutate: func(*TopQueryCollection) {},
		},
		{
			desc:      "top_query_count zero is invalid",
			mutate:    func(c *TopQueryCollection) { c.TopQueryCount = 0 },
			errSubstr: "top_query_collection.top_query_count must be greater than 0",
		},
		{
			desc:      "top_query_count negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.TopQueryCount = -1 },
			errSubstr: "top_query_collection.top_query_count must be greater than 0",
		},
		{
			desc:      "max_query_sample_count zero is invalid",
			mutate:    func(c *TopQueryCollection) { c.MaxQuerySampleCount = 0 },
			errSubstr: "top_query_collection.max_query_sample_count must be greater than 0",
		},
		{
			desc:      "max_query_sample_count negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.MaxQuerySampleCount = -5 },
			errSubstr: "top_query_collection.max_query_sample_count must be greater than 0",
		},
		{
			desc:   "max_explain_each_interval zero is valid (disables explain)",
			mutate: func(c *TopQueryCollection) { c.MaxExplainEachInterval = 0 },
		},
		{
			desc:      "max_explain_each_interval negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.MaxExplainEachInterval = -1 },
			errSubstr: "top_query_collection.max_explain_each_interval must not be negative",
		},
		{
			desc:   "query_plan_cache_size zero is valid (disables caching)",
			mutate: func(c *TopQueryCollection) { c.QueryPlanCacheSize = 0 },
		},
		{
			desc:      "query_plan_cache_size negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.QueryPlanCacheSize = -1 },
			errSubstr: "top_query_collection.query_plan_cache_size must not be negative",
		},
		{
			desc:      "collection_interval zero is invalid",
			mutate:    func(c *TopQueryCollection) { c.CollectionInterval = 0 },
			errSubstr: "top_query_collection.collection_interval must be greater than 0",
		},
		{
			desc:      "collection_interval negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.CollectionInterval = -time.Second },
			errSubstr: "top_query_collection.collection_interval must be greater than 0",
		},
		{
			desc:      "query_plan_cache_ttl zero is invalid",
			mutate:    func(c *TopQueryCollection) { c.QueryPlanCacheTTL = 0 },
			errSubstr: "top_query_collection.query_plan_cache_ttl must be greater than 0",
		},
		{
			desc:      "query_plan_cache_ttl negative is invalid",
			mutate:    func(c *TopQueryCollection) { c.QueryPlanCacheTTL = -time.Second },
			errSubstr: "top_query_collection.query_plan_cache_ttl must be greater than 0",
		},
		{
			desc: "top_query_count larger than max_query_sample_count is valid (independent knobs)",
			mutate: func(c *TopQueryCollection) {
				c.MaxQuerySampleCount = 10
				c.TopQueryCount = 50
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := baseCfg()
			tc.mutate(&cfg.TopQueryCollection)
			err := xconfmap.Validate(cfg)
			if tc.errSubstr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.errSubstr)
			}
		})
	}
}

// TestValidateTopQueryCollection_PartialUserConfig exercises the mapstructure
// merge path: a user's partial `top_query_collection` block should inherit
// unspecified fields from createDefaultConfig, so validation still passes.
func TestValidateTopQueryCollection_PartialUserConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	// Simulate a user who only overrides top_query_count via mapstructure
	// decoding into the already-populated struct returned by createDefaultConfig.
	partial := map[string]any{
		"top_query_collection": map[string]any{
			"top_query_count": int64(50),
		},
	}
	require.NoError(t, decodeInto(cfg, partial))
	require.Equal(t, int64(50), cfg.TopQueryCollection.TopQueryCount)
	// All other fields keep their factory defaults.
	require.Equal(t, defaultMaxQuerySampleCount, cfg.TopQueryCollection.MaxQuerySampleCount)
	require.Equal(t, defaultMaxExplainEachInterval, cfg.TopQueryCollection.MaxExplainEachInterval)
	require.Equal(t, defaultQueryPlanCacheSize, cfg.TopQueryCollection.QueryPlanCacheSize)
	require.Equal(t, defaultQueryPlanCacheTTL, cfg.TopQueryCollection.QueryPlanCacheTTL)
	require.Equal(t, defaultTopQueryCollectionInterval, cfg.TopQueryCollection.CollectionInterval)

	require.NoError(t, xconfmap.Validate(cfg))
}

// decodeInto merges a raw config map into an already-populated Config the
// same way the collector runtime does, exercising the mapstructure defaults
// preservation path.
func decodeInto(cfg *Config, raw map[string]any) error {
	return confmap.NewFromStringMap(raw).Unmarshal(cfg)
}
