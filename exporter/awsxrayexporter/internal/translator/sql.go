// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"go.opentelemetry.io/collector/model/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeSQL(attributes map[string]pdata.AttributeValue) (map[string]pdata.AttributeValue, *awsxray.SQLData) {
	var (
		filtered    = make(map[string]pdata.AttributeValue)
		sqlData     awsxray.SQLData
		dbURL       string
		dbSystem    string
		dbInstance  string
		dbStatement string
		dbUser      string
	)

	for key, value := range attributes {
		switch key {
		case semconventions.AttributeDBConnectionString:
			dbURL = value.StringVal()
		case semconventions.AttributeDBSystem:
			dbSystem = value.StringVal()
		case semconventions.AttributeDBName:
			dbInstance = value.StringVal()
		case semconventions.AttributeDBStatement:
			dbStatement = value.StringVal()
		case semconventions.AttributeDBUser:
			dbUser = value.StringVal()
		default:
			filtered[key] = value
		}
	}

	if !isSQL(dbSystem) {
		// Either no DB attributes or this is not an SQL DB.
		return attributes, nil
	}

	if dbURL == "" {
		dbURL = "localhost"
	}
	url := dbURL + "/" + dbInstance
	sqlData = awsxray.SQLData{
		URL:            awsxray.String(url),
		DatabaseType:   awsxray.String(dbSystem),
		User:           awsxray.String(dbUser),
		SanitizedQuery: awsxray.String(dbStatement),
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
