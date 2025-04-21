// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type (
	clusterTestCompletion func(t *testing.T, dsn string, clusterTest clusterTestConfig, fns ...func(*Config))
	clusterTestConfig     struct {
		name       string
		cluster    string
		shouldPass bool
	}
)

func withDriverName(driverName string) func(*Config) {
	return func(c *Config) {
		c.driverName = driverName
	}
}

func (test clusterTestConfig) verifyConfig(t *testing.T, cfg *Config) {
	if test.cluster == "" {
		require.Empty(t, cfg.clusterString())
	} else {
		require.NotEmpty(t, cfg.clusterString())
	}
}

func getQueryFirstLine(query string) string {
	trimmed := strings.Trim(query, "\n")
	line := strings.Split(trimmed, "\n")[0]
	return strings.Trim(line, " (")
}

func checkClusterQueryDefinition(query string, clusterName string) error {
	line := getQueryFirstLine(query)
	lowercasedLine := strings.ToLower(line)
	suffix := fmt.Sprintf("ON CLUSTER %s", clusterName)
	prefixes := []string{"create database", "create table", "create materialized view"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lowercasedLine, prefix) {
			if strings.HasSuffix(line, suffix) {
				return nil
			}
		}
	}

	return fmt.Errorf("query does not contain cluster clause: %s", line)
}

func testClusterConfig(t *testing.T, completion clusterTestCompletion) {
	tests := []clusterTestConfig{
		{
			name:       "on",
			cluster:    "cluster_a_b",
			shouldPass: true,
		},
		{
			name:       "off",
			cluster:    "",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run("test cluster config "+tt.name, func(t *testing.T) {
			initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
				if tt.shouldPass {
					require.NoError(t, checkClusterQueryDefinition(query, tt.cluster))
				} else {
					require.Error(t, checkClusterQueryDefinition(query, tt.cluster))
				}
				return nil
			})

			var configMods []func(*Config)
			configMods = append(configMods, func(cfg *Config) {
				cfg.ClusterName = tt.cluster
				cfg.Database = "test_db_" + time.Now().Format("20060102150405")
			})

			completion(t, defaultEndpoint, tt, configMods...)
		})
	}
}

type (
	tableEngineTestCompletion func(t *testing.T, dsn string, engineTest tableEngineTestConfig, fns ...func(*Config))
	tableEngineTestConfig     struct {
		name              string
		engineName        string
		engineParams      string
		expectedTableName string
		shouldPass        bool
	}
)

func (engineTest tableEngineTestConfig) verifyConfig(t *testing.T, te TableEngine) {
	if engineTest.engineName == "" {
		require.Empty(t, te.Name)
	} else {
		require.NotEmpty(t, te.Name)
	}
}

func checkTableEngineQueryDefinition(query string, expectedEngineName string) error {
	lines := strings.Split(query, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "engine = ") {
			engine := strings.Split(line, " = ")[1]
			engine = strings.Trim(engine, " ")
			if engine == expectedEngineName {
				return nil
			}

			return fmt.Errorf("wrong engine name in query: %s, expected: %s", engine, expectedEngineName)
		}
	}

	return fmt.Errorf("query does not contain engine definition: %s", query)
}

func testTableEngineConfig(t *testing.T, completion tableEngineTestCompletion) {
	tests := []tableEngineTestConfig{
		{
			name:              "no params",
			engineName:        "CustomEngine",
			engineParams:      "",
			expectedTableName: "CustomEngine",
			shouldPass:        true,
		},
		{
			name:              "with params",
			engineName:        "CustomEngine",
			engineParams:      "'/x/y/z', 'some_param', another_param, last_param",
			expectedTableName: "CustomEngine",
			shouldPass:        true,
		},
		{
			name:              "with empty name",
			engineName:        "",
			engineParams:      "",
			expectedTableName: defaultTableEngineName,
			shouldPass:        true,
		},
		{
			name:              "fail",
			engineName:        "CustomEngine",
			engineParams:      "",
			expectedTableName: defaultTableEngineName,
			shouldPass:        false,
		},
	}

	for _, tt := range tests {
		te := TableEngine{Name: tt.engineName, Params: tt.engineParams}
		expectedEngineValue := fmt.Sprintf("%s(%s)", tt.expectedTableName, tt.engineParams)

		t.Run("test table engine config "+tt.name, func(t *testing.T) {
			initClickhouseTestServer(t, func(query string, _ []driver.Value) error {
				firstLine := getQueryFirstLine(query)
				if !strings.HasPrefix(strings.ToLower(firstLine), "create table") {
					return nil
				}

				check := checkTableEngineQueryDefinition(query, expectedEngineValue)
				if tt.shouldPass {
					require.NoError(t, check)
				} else {
					require.Error(t, check)
				}

				return nil
			})

			var configMods []func(*Config)
			if te.Name != "" {
				configMods = append(configMods, func(cfg *Config) {
					cfg.TableEngine = te
				})
			}

			completion(t, defaultEndpoint, tt, configMods...)
		})
	}
}
