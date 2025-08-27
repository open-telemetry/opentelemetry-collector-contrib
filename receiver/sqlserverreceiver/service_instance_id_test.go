// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
)

func getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func TestComputeServiceInstanceID(t *testing.T) {
	hostname := getHostname()

	tests := []struct {
		name     string
		config   *Config
		expected string
		wantErr  bool
	}{
		{
			name: "explicit_server_and_port",
			config: &Config{
				Server: "myserver",
				Port:   5000,
			},
			expected: "myserver:5000",
		},
		{
			name: "explicit_server_default_port_zero",
			config: &Config{
				Server: "myserver",
				Port:   0,
			},
			expected: "myserver:1433",
		},
		{
			name: "explicit_server_default_port_1433",
			config: &Config{
				Server: "myserver",
				Port:   1433,
			},
			expected: "myserver:1433",
		},
		{
			name: "datasource_with_port_comma_separator",
			config: &Config{
				DataSource: "server=myserver,5000;user id=sa;password=pass",
			},
			expected: "myserver:5000",
		},
		{
			name: "datasource_with_port_colon_separator",
			config: &Config{
				DataSource: "server=myserver:5000;user id=sa;password=pass",
			},
			expected: "myserver:5000",
		},
		{
			name: "datasource_without_port",
			config: &Config{
				DataSource: "server=myserver;user id=sa;password=pass",
			},
			expected: "myserver:1433",
		},
		{
			name: "datasource_with_data_source_keyword",
			config: &Config{
				DataSource: "Data Source=myserver,5000;Initial Catalog=mydb",
			},
			expected: "myserver:5000",
		},
		{
			name: "datasource_with_separate_port_param",
			config: &Config{
				DataSource: "server=myserver;port=5000;user id=sa",
			},
			expected: "myserver:5000",
		},
		{
			name: "localhost_replacement_with_explicit_server",
			config: &Config{
				Server: "localhost",
				Port:   1433,
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "localhost_127_0_0_1_replacement",
			config: &Config{
				Server: "127.0.0.1",
				Port:   1433,
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "localhost_in_datasource",
			config: &Config{
				DataSource: "server=localhost;user id=sa",
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "no_server_uses_hostname",
			config: &Config{},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "no_server_no_datasource",
			config: &Config{
				Username: "sa",
				Password: configopaque.String("pass"),
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "named_instance_with_port",
			config: &Config{
				Server: "myserver\\SQLEXPRESS",
				Port:   5000,
			},
			expected: "myserver\\SQLEXPRESS:5000",
		},
		{
			name: "datasource_with_named_instance",
			config: &Config{
				DataSource: "server=myserver\\SQLEXPRESS,5000;user id=sa",
			},
			expected: "myserver\\SQLEXPRESS:5000",
		},
		{
			name: "datasource_with_spaces_around_equals",
			config: &Config{
				DataSource: "server = myserver , 5000 ; user id = sa",
			},
			expected: "myserver:5000",
		},
		{
			name: "case_insensitive_datasource_keywords",
			config: &Config{
				DataSource: "SERVER=MyServer,5000;User Id=sa",
			},
			expected: "MyServer:5000",
		},
		{
			name: "invalid_datasource_no_server",
			config: &Config{
				DataSource: "user id=sa;password=pass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := computeServiceInstanceID(tt.config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestParseDataSource(t *testing.T) {
	tests := []struct {
		name         string
		dataSource   string
		expectedHost string
		expectedPort int
		wantErr      bool
	}{
		{
			name:         "standard_comma_separator",
			dataSource:   "server=myserver,5000;user id=sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "colon_separator",
			dataSource:   "server=myserver:5000;user id=sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "no_port_specified",
			dataSource:   "server=myserver;user id=sa",
			expectedHost: "myserver",
			expectedPort: 1433,
		},
		{
			name:         "separate_port_parameter",
			dataSource:   "server=myserver;port=5000;user id=sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "data_source_keyword",
			dataSource:   "Data Source=myserver,5000",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "named_instance",
			dataSource:   "server=myserver\\SQLEXPRESS,5000",
			expectedHost: "myserver\\SQLEXPRESS",
			expectedPort: 5000,
		},
		{
			name:         "spaces_in_connection_string",
			dataSource:   "server = myserver , 5000 ; user id = sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "preserve_case_of_server_name",
			dataSource:   "Server=MyServerName,5000",
			expectedHost: "MyServerName",
			expectedPort: 5000,
		},
		{
			name:         "no_server_in_datasource",
			dataSource:   "user id=sa;password=pass",
			wantErr:      true,
		},
		{
			name:         "empty_datasource",
			dataSource:   "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseDataSource(tt.dataSource)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
				assert.Equal(t, tt.expectedPort, port)
			}
		})
	}
}