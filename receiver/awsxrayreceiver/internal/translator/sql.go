// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventionsv125 "go.opentelemetry.io/otel/semconv/v1.25.0"
	conventionsv128 "go.opentelemetry.io/otel/semconv/v1.28.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addSQLToSpan(sql *awsxray.SQLData, attrs pcommon.Map) error {
	if sql == nil {
		return nil
	}

	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c615d2db351929b99e46f7b427f39c12afe15b54/exporter/awsxrayexporter/translator/sql.go#L60
	if sql.URL != nil {
		dbURL, dbName, err := splitSQLURL(*sql.URL)
		if err != nil {
			return err
		}
		attrs.PutStr(string(conventionsv125.DBConnectionStringKey), dbURL)
		attrs.PutStr(string(conventionsv125.DBNameKey), dbName)
	}
	// not handling sql.ConnectionString for now because the X-Ray exporter
	// does not support it
	addString(sql.DatabaseType, string(conventionsv128.DBSystemKey), attrs)
	addString(sql.SanitizedQuery, string(conventionsv125.DBStatementKey), attrs)
	addString(sql.User, string(conventionsv125.DBUserKey), attrs)
	return nil
}

// SQL URL is of the format: protocol+transport://host:port/dbName?queryParam
var re = regexp.MustCompile(`^(.+//.+)/([^\?]+)\??.*$`)

const (
	dbURLI  = 1
	dbNameI = 2
)

func splitSQLURL(rawURL string) (string, string, error) {
	m := re.FindStringSubmatch(rawURL)
	if len(m) == 0 {
		return "", "", fmt.Errorf(
			"failed to parse out the database name in the \"sql.url\" field, rawUrl: %s",
			rawURL,
		)
	}
	return m[dbURLI], m[dbNameI], nil
}
