// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

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
		case conventions.AttributeDBConnectionString:
			dbConnectionString = value.Str()
		case conventions.AttributeDBSystem:
			dbSystem = value.Str()
		case conventions.AttributeDBName:
			dbInstance = value.Str()
		case conventions.AttributeDBStatement:
			dbStatement = value.Str()
		case conventions.AttributeDBUser:
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
