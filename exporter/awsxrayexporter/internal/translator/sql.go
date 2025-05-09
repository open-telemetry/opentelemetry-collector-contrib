// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventionsv112 "go.opentelemetry.io/otel/semconv/v1.12.0"

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
		case string(conventionsv112.DBConnectionStringKey):
			dbConnectionString = value.Str()
		case string(conventionsv112.DBSystemKey):
			dbSystem = value.Str()
		case string(conventionsv112.DBNameKey):
			dbInstance = value.Str()
		case string(conventionsv112.DBStatementKey):
			dbStatement = value.Str()
		case string(conventionsv112.DBUserKey):
			dbUser = value.Str()
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
	case "db2":
		fallthrough
	case "derby":
		fallthrough
	case "hive":
		fallthrough
	case "mariadb":
		fallthrough
	case "mssql":
		fallthrough
	case "mysql":
		fallthrough
	case "oracle":
		fallthrough
	case "postgresql":
		fallthrough
	case "sqlite":
		fallthrough
	case "teradata":
		fallthrough
	case "other_sql":
		return true
	default:
	}
	return false
}
