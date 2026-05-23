// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mysqlreceiver

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const mysqlPort = "3306"

type mySQLTestConfig struct {
	name         string
	containerCmd []string
	tlsEnabled   bool
	insecureSkip bool
	imageVersion string
	expectedFile string
}

func TestIntegration(t *testing.T) {
	testCases := []mySQLTestConfig{
		{
			name:         "MySql-8.0.33-WithoutTLS",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mysql:8.0.33",
			expectedFile: "expected-mysql.yaml",
		},
		{
			name:         "MySql-8.0.33-WithTLS",
			containerCmd: []string{"--auto_generate_certs=ON", "--require_secure_transport=ON"},
			tlsEnabled:   true,
			insecureSkip: true,
			imageVersion: "mysql:8.0.33",
			expectedFile: "expected-mysql.yaml",
		},
		{
			name:         "MariaDB-11.6.2",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mariadb:11.6.2-ubi9",
			expectedFile: "expected-mariadb.yaml",
		},
		{
			name:         "MariaDB-10.11.11",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mariadb:10.11.11-ubi9",
			expectedFile: "expected-mariadb.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraperinttest.NewIntegrationTest(
				NewFactory(),
				scraperinttest.WithContainerRequest(
					testcontainers.ContainerRequest{
						Image:        tc.imageVersion,
						Cmd:          tc.containerCmd,
						ExposedPorts: []string{mysqlPort},
						WaitingFor: wait.ForListeningPort(mysqlPort).
							WithStartupTimeout(2 * time.Minute),
						Env: map[string]string{
							"MYSQL_ROOT_PASSWORD": "otel",
							"MYSQL_DATABASE":      "otel",
							"MYSQL_USER":          "otel",
							"MYSQL_PASSWORD":      "otel",
						},
						Files: []testcontainers.ContainerFile{
							{
								HostFilePath:      filepath.Join("testdata", "integration", "init.sh"),
								ContainerFilePath: "/docker-entrypoint-initdb.d/init.sh",
								FileMode:          700,
							},
						},
					}),
				scraperinttest.WithCustomConfig(
					func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
						rCfg := cfg.(*Config)
						rCfg.CollectionInterval = time.Second
						rCfg.Endpoint = net.JoinHostPort(ci.Host(t), ci.MappedPort(t, mysqlPort))
						rCfg.Username = "otel"
						rCfg.Password = "otel"
						if tc.tlsEnabled {
							rCfg.TLS.InsecureSkipVerify = tc.insecureSkip
						} else {
							rCfg.TLS.Insecure = true
						}
					}),
				scraperinttest.WithExpectedFile(
					filepath.Join("testdata", "integration", tc.expectedFile),
				),
				scraperinttest.WithCompareOptions(
					pmetrictest.IgnoreResourceAttributeValue("mysql.instance.endpoint"),
					pmetrictest.IgnoreMetricValues(),
					pmetrictest.IgnoreMetricDataPointsOrder(),
					pmetrictest.IgnoreStartTimestamp(),
					pmetrictest.IgnoreTimestamp(),
				),
			).Run(t)
		})
	}
}
