// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeSQL(span ptrace.Span, attributes map[string]pcommon.Value) (map[string]pcommon.Value, *awsxray.SQLData) {
	var (
		filtered           = make(map[string]pcommon.Value)
		sqlData            awsxray.SQLData
		dbURL              string
		dbConnectionString string
		dbSystem           string
		dbInstance         string
		dbStatement        string
		dbUser             string
	)

	for key, value := range attributes {
		switch key {
		// New v1.38.0 keys — gate-guarded by EmitV1DB:
		case string(conventions.DBNamespaceKey):
			if metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate.IsEnabled() {
				dbInstance = value.Str()
			} else {
				filtered[key] = value
			}
		case string(conventions.DBSystemNameKey):
			if metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate.IsEnabled() {
				dbSystem = value.Str()
			} else {
				filtered[key] = value
			}
		case string(conventions.DBQueryTextKey):
			if metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate.IsEnabled() {
				dbStatement = value.Str()
			} else {
				filtered[key] = value
			}
		case string(conventions.UserNameKey):
			if metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate.IsEnabled() {
				dbUser = value.Str()
			} else {
				filtered[key] = value
			}
		// Old v1.12.0 keys — gate-guarded:
		// TODO: Remove these cases when exporter.awsxray.DontEmitV0DBConventions is removed.
		case "db.connection_string":
			if !metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate.IsEnabled() {
				dbConnectionString = value.Str()
			} else {
				filtered[key] = value
			}
		case "db.name":
			if !metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate.IsEnabled() {
				dbInstance = value.Str()
			} else {
				filtered[key] = value
			}
		case "db.statement":
			if !metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate.IsEnabled() {
				dbStatement = value.Str()
			} else {
				filtered[key] = value
			}
		case "db.system":
			if !metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate.IsEnabled() {
				dbSystem = value.Str()
			} else {
				filtered[key] = value
			}
		case "db.user":
			if !metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate.IsEnabled() {
				dbUser = value.Str()
			} else {
				filtered[key] = value
			}
		default:
			filtered[key] = value
		}
	}

	if !isSQL(dbSystem) {
		// Either no DB attributes or this is not an SQL DB.
		return attributes, nil
	}

	// Despite what the X-Ray documents say, having the DB connection string
	// set as the URL value of the segment is not useful. So let's use the
	// current span name instead
	dbURL = span.Name()

	// Let's keep the original format for connection_string
	if dbConnectionString == "" {
		dbConnectionString = "localhost"
	}
	dbConnectionString = dbConnectionString + "/" + dbInstance

	sqlData = awsxray.SQLData{
		URL:              awsxray.String(dbURL),
		ConnectionString: awsxray.String(dbConnectionString),
		DatabaseType:     awsxray.String(dbSystem),
		User:             awsxray.String(dbUser),
		SanitizedQuery:   awsxray.String(dbStatement),
	}
	return filtered, &sqlData
}

func isSQL(system string) bool {
	switch system {
	// v1.12.0 values (default):
	// TODO: Remove v1.12.0 system name values when exporter.awsxray.DontEmitV0DBConventions is removed.
	case "db2", "mssql", "oracle":
		return true
	// Unchanged values (present in both v1.12.0 and v1.38.0):
	case "derby",
		"hive",
		"mariadb",
		"mysql",
		"other_sql",
		"postgresql",
		"sqlite",
		"teradata":
		return true
	// v1.38.0 values (only when EmitV1DB is enabled):
	// TODO: Remove this gate when exporter.awsxray.DontEmitV0DBConventions is removed.
	case conventions.DBSystemNameIBMDB2.Value.AsString(),
		conventions.DBSystemNameMicrosoftSQLServer.Value.AsString(),
		conventions.DBSystemNameOracleDB.Value.AsString():
		return metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate.IsEnabled()
	default:
	}
	return false
}
