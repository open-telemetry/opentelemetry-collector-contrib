// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		t.Context(),
		receiver.Settings{
			ID:                component.NewID(metadata.Type),
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		factory.CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	config := factory.CreateDefaultConfig().(*Config)
	_, logsErr := factory.CreateLogs(
		t.Context(),
		receiver.Settings{
			ID:                component.NewID(metadata.Type),
			TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		},
		config,

		consumertest.NewNop(),
	)
	require.NoError(t, logsErr)
}

func TestGetInstanceName(t *testing.T) {
	instanceName, err := getInstanceName("oracle://example.com:1521/mydb")
	assert.NoError(t, err)
	assert.Equal(t, "example.com:1521/mydb", instanceName)

	// Should fail on non-encoded special characters
	_, err = getInstanceName("oracle://username1:p@ssw%rd@example1.com:1521/mydb")
	assert.ErrorContains(t, err, "invalid URL escape")

	// Should succeed when special characters are encoded
	instanceName, err = getInstanceName("oracle://username1:p@ssword%25-_1@example1.com:1521/mydb")
	assert.NoError(t, err)
	assert.Equal(t, "example1.com:1521/mydb", instanceName)
}

// TestResolveServerEndpointFromDataSource covers every datasource form end to end, getHostName included.
func TestResolveServerEndpointFromDataSource(t *testing.T) {
	localhostName, err := os.Hostname()
	require.NoError(t, err)

	tests := []struct {
		name            string
		datasource      string
		expectedAddress string
		expectedPort    int64
	}{
		{name: "loopback name with port", datasource: "oracle://otel:password@localhost:51521/XE", expectedAddress: localhostName, expectedPort: 51521},
		{name: "loopback name without port", datasource: "oracle://otel:password@localhost/XE", expectedAddress: localhostName, expectedPort: 1521},
		{name: "IPv4 loopback with port", datasource: "oracle://otel:password@127.0.0.1:1521/XE", expectedAddress: localhostName, expectedPort: 1521},
		{name: "IPv4 loopback without port", datasource: "oracle://otel:password@127.0.0.1/XE", expectedAddress: localhostName, expectedPort: 1521},
		{name: "IPv6 loopback with port", datasource: "oracle://otel:password@[::1]:1521/XE", expectedAddress: localhostName, expectedPort: 1521},
		{name: "IPv6 loopback without port", datasource: "oracle://otel:password@[::1]/XE", expectedAddress: localhostName, expectedPort: 1521},
		{name: "container hostname is left alone", datasource: "oracle://otel:password@ora-docker:1521/XE", expectedAddress: "ora-docker", expectedPort: 1521},
		{name: "docker host gateway is left alone", datasource: "oracle://otel:password@host.docker.internal:1521/XE", expectedAddress: "host.docker.internal", expectedPort: 1521},
		{name: "remote host without port", datasource: "oracle://otel:password@example.com/XE", expectedAddress: "example.com", expectedPort: 1521},
		// url.Parse reads neither form, so the host is undetermined and never reported as empty.
		{name: "TNS descriptor", datasource: "(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=XE)))", expectedAddress: localhostName, expectedPort: 1521},
		{name: "Easy Connect without the oracle prefix", datasource: "otel/password@localhost:51521/XE", expectedAddress: localhostName, expectedPort: 1521},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hostName, hostNameErr := getHostName(test.datasource)
			require.NoError(t, hostNameErr)

			address, port := resolveServerEndpoint(hostName, zap.NewNop())
			assert.Equal(t, test.expectedAddress, address)
			assert.Equal(t, test.expectedPort, port)
		})
	}
}

// TestServerEndpointAndInstanceIDAgree guards against server.address/port drifting from service.instance.id.
func TestServerEndpointAndInstanceIDAgree(t *testing.T) {
	datasources := []string{
		"oracle://otel:password@localhost:51521/XE",
		"oracle://otel:password@localhost/XE",
		"oracle://otel:password@127.0.0.1/XE",
		"oracle://otel:password@[::1]/XE",
		"oracle://otel:password@example.com/XE",
		"oracle://otel:password@ora-docker:1521/XE",
		"(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=XE)))",
	}

	for _, datasource := range datasources {
		t.Run(datasource, func(t *testing.T) {
			hostName, err := getHostName(datasource)
			require.NoError(t, err)
			instanceName, err := getInstanceName(datasource)
			require.NoError(t, err)

			address, port, instanceID := resolveInstanceIdentity(hostName, instanceName, zap.NewNop())

			assert.True(t, strings.HasPrefix(instanceID, address+":"+strconv.FormatInt(port, 10)),
				"service.instance.id %q must start with the resolved endpoint %s:%d", instanceID, address, port)
		})
	}
}

// TestResolveServerEndpointFromEndpoint covers the secondary configuration option (endpoint/service).
func TestResolveServerEndpointFromEndpoint(t *testing.T) {
	localhostName, err := os.Hostname()
	require.NoError(t, err)

	cfg := Config{
		Endpoint: "localhost:51521",
		Password: "password",
		Service:  "XE",
		Username: "otel",
	}

	hostName, hostNameErr := getHostName(getDataSource(cfg))
	require.NoError(t, hostNameErr)

	address, port := resolveServerEndpoint(hostName, zap.NewNop())
	assert.Equal(t, localhostName, address)
	assert.Equal(t, int64(51521), port)
}

func TestGetDataSource(t *testing.T) {
	endpoint := "example1.com:1521"
	password := "p@ssword%-_1"
	service := "mydb1"
	username := "username1"
	nonDefaultDataSource := "oracle://username1:p@ssword%25-_1@example1.com:1521/mydb1"
	defaultDataSource := "oracle://username:password@example.com:1521/mydb"

	testCases := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name: "Default data source",
			config: &Config{
				DataSource: defaultDataSource,
			},
			expected: defaultDataSource,
		},
		{
			name: "Default data source takes priority over other config options",
			config: &Config{
				DataSource: defaultDataSource,
				Endpoint:   endpoint,
				Password:   password,
				Service:    service,
				Username:   username,
			},
			expected: defaultDataSource,
		},
		{
			name: "Individual config options properly render data source",
			config: &Config{
				Endpoint: endpoint,
				Password: password,
				Service:  service,
				Username: username,
			},
			expected: nonDefaultDataSource,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataSource := getDataSource(*tc.config)
			require.Equal(t, tc.expected, dataSource)
			_, err := url.PathUnescape(dataSource)
			require.NoError(t, err)
		})
	}
}
