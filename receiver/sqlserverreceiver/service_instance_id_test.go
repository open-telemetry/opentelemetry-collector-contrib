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

func getTestHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

func TestComputeServiceInstanceID(t *testing.T) {
	hostname := getTestHostname()

	tests := []struct {
		name     string
		config   *Config
		expected string
		wantErr  bool
	}{
		{
			name: "explicit server and port",
			config: &Config{
				Server: "myserver",
				Port:   5000,
			},
			expected: "myserver:5000",
		},
		{
			name: "explicit server default port zero",
			config: &Config{
				Server: "myserver",
				Port:   0,
			},
			expected: "myserver:1433",
		},
		{
			name: "explicit server default port 1433",
			config: &Config{
				Server: "myserver",
				Port:   1433,
			},
			expected: "myserver:1433",
		},
		{
			name: "datasource with port comma separator",
			config: &Config{
				DataSource: "server=myserver,5000;user id=sa;password=pass",
			},
			expected: "myserver:5000",
		},
		{
			name: "datasource with port colon separator",
			config: &Config{
				DataSource: "server=myserver:5000;user id=sa;password=pass",
			},
			expected: "myserver:5000:1433", // msdsn treats "myserver:5000" as the host name
		},
		{
			name: "datasource without port",
			config: &Config{
				DataSource: "server=myserver;user id=sa;password=pass",
			},
			expected: "myserver:1433",
		},
		{
			name: "datasource with data source keyword",
			config: &Config{
				DataSource: "Data Source=myserver,5000;Initial Catalog=mydb",
			},
			expected: "myserver:5000",
		},
		{
			name: "datasource with separate port param",
			config: &Config{
				DataSource: "server=myserver;port=5000;user id=sa",
			},
			expected: "myserver:5000",
		},
		{
			name: "localhost replacement with explicit server",
			config: &Config{
				Server: "localhost",
				Port:   1433,
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "localhost 127.0.0.1 replacement",
			config: &Config{
				Server: "127.0.0.1",
				Port:   1433,
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "localhost in datasource",
			config: &Config{
				DataSource: "server=localhost;user id=sa",
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "empty server in datasource",
			config: &Config{
				DataSource: "user id=sa",
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name:     "no server uses hostname",
			config:   &Config{},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "no server no datasource",
			config: &Config{
				Username: "sa",
				Password: configopaque.String("pass"),
			},
			expected: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "datasource with named instance",
			config: &Config{
				DataSource: "server=myserver\\SQLEXPRESS,5000;user id=sa",
			},
			expected: "myserver:5000", // Instance name not included in service.instance.id
		},
		{
			name: "datasource with spaces around equals",
			config: &Config{
				DataSource: "server = myserver , 5000 ; user id = sa",
			},
			wantErr: true, // msdsn cannot parse spaces around comma in port
		},
		{
			name: "case insensitive datasource keywords",
			config: &Config{
				DataSource: "SERVER=MyServer,5000;User Id=sa",
			},
			expected: "MyServer:5000",
		},
		{
			name: "invalid datasource no server defaults to localhost",
			config: &Config{
				DataSource: "user id=sa;password=pass",
			},
			expected: fmt.Sprintf("%s:1433", hostname), // msdsn defaults to localhost when no server specified
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
			name:         "standard comma separator",
			dataSource:   "server=myserver,5000;user id=sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "colon separator treated as host",
			dataSource:   "server=myserver:5000;user id=sa",
			expectedHost: "myserver:5000", // msdsn treats this as host
			expectedPort: 1433,
		},
		{
			name:         "no port specified",
			dataSource:   "server=myserver;user id=sa",
			expectedHost: "myserver",
			expectedPort: 1433,
		},
		{
			name:         "separate port parameter",
			dataSource:   "server=myserver;port=5000;user id=sa",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "data source keyword",
			dataSource:   "Data Source=myserver,5000",
			expectedHost: "myserver",
			expectedPort: 5000,
		},
		{
			name:         "named instance",
			dataSource:   "server=myserver\\SQLEXPRESS,5000",
			expectedHost: "myserver", // Instance name not included
			expectedPort: 5000,
		},
		{
			name:       "spaces in connection string",
			dataSource: "server = myserver , 5000 ; user id = sa",
			wantErr:    true, // msdsn cannot parse spaces around comma
		},
		{
			name:         "preserve case of server name",
			dataSource:   "Server=MyServerName,5000",
			expectedHost: "MyServerName",
			expectedPort: 5000,
		},
		{
			name:         "no server in datasource defaults to localhost",
			dataSource:   "user id=sa;password=pass",
			expectedHost: "localhost", // msdsn defaults to localhost
			expectedPort: 1433,
		},
		{
			name:       "empty datasource",
			dataSource: "",
			wantErr:    true,
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
