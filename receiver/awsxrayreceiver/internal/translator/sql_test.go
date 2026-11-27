// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
)

func TestAddSQLToSpanDatabaseConventions(t *testing.T) {
	// Not parallel: mutates the global feature-gate registry.
	ptr := func(value string) *string { return &value }
	sql := &awsxray.SQLData{
		URL:            ptr("jdbc:postgresql://db.example.com:5432/ebdb"),
		SanitizedQuery: ptr("SELECT * FROM users"),
		DatabaseType:   ptr("postgresql"),
		User:           ptr("db-user"),
	}

	tests := []struct {
		name       string
		dontEmitV0 bool
		emitV1     bool
		want       map[string]any
	}{
		{
			name: "legacy only",
			want: map[string]any{
				"db.connection_string": "jdbc:postgresql://db.example.com:5432",
				"db.name":              "ebdb",
				"db.system":            "postgresql",
				"db.statement":         "SELECT * FROM users",
				"db.user":              "db-user",
			},
		},
		{
			name:   "legacy and current",
			emitV1: true,
			want: map[string]any{
				"db.connection_string": "jdbc:postgresql://db.example.com:5432",
				"db.name":              "ebdb",
				"db.system":            "postgresql",
				"db.statement":         "SELECT * FROM users",
				"db.user":              "db-user",
				"db.namespace":         "ebdb",
				"db.system.name":       "postgresql",
				"db.query.text":        "SELECT * FROM users",
			},
		},
		{
			name:       "current only",
			dontEmitV0: true,
			emitV1:     true,
			want: map[string]any{
				"db.namespace":   "ebdb",
				"db.system.name": "postgresql",
				"db.query.text":  "SELECT * FROM users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := featuregate.GlobalRegistry()
			require.NoError(t, registry.Set(metadata.ReceiverAwsxrayDontEmitV0DatabaseConventionsFeatureGate.ID(), tt.dontEmitV0))
			require.NoError(t, registry.Set(metadata.ReceiverAwsxrayEmitV1DatabaseConventionsFeatureGate.ID(), tt.emitV1))
			t.Cleanup(func() {
				require.NoError(t, registry.Set(metadata.ReceiverAwsxrayDontEmitV0DatabaseConventionsFeatureGate.ID(), false))
				require.NoError(t, registry.Set(metadata.ReceiverAwsxrayEmitV1DatabaseConventionsFeatureGate.ID(), false))
			})

			attrs := pcommon.NewMap()
			require.NoError(t, addSQLToSpan(sql, attrs))
			require.Equal(t, tt.want, attrs.AsRaw())
		})
	}
}

func TestSQLURL(t *testing.T) {
	raw := "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432/ebdb"
	url, dbName, err := splitSQLURL(raw)
	assert.NoError(t, err, "should succeed")
	assert.Equal(t,
		"jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432",
		url, "expected url to be the same")

	assert.Equal(t,
		"ebdb",
		dbName, "expected db name to be the same")
}

func TestSQLURLQueryParameter(t *testing.T) {
	raw := "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432/ebdb?myInterceptor=foo"
	url, dbName, err := splitSQLURL(raw)
	assert.NoError(t, err, "should succeed")
	assert.Equal(t,
		"jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432",
		url, "expected url to be the same")

	assert.Equal(t,
		"ebdb",
		dbName, "expected db name to be the same")
}

func TestFsURL(t *testing.T) {
	raw := "jdbc:sqlite:/tmp/ebdb.sqlite"
	url, dbName, err := splitSQLURL(raw)
	assert.NoError(t, err, "should succeed")
	assert.Equal(t,
		"jdbc:sqlite:",
		url, "expected url to be the same")

	assert.Equal(t,
		"/tmp/ebdb.sqlite",
		dbName, "expected db name to be the same")
}
