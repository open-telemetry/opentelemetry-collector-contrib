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
	semconventions "go.opentelemetry.io/collector/translator/conventions"
)

// SQLData provides the shape for unmarshalling sql data.
type SQLData struct {
	ConnectionString string `json:"connection_string,omitempty"`
	URL              string `json:"url,omitempty"` // host:port/database
	DatabaseType     string `json:"database_type,omitempty"`
	DatabaseVersion  string `json:"database_version,omitempty"`
	DriverVersion    string `json:"driver_version,omitempty"`
	User             string `json:"user,omitempty"`
	Preparation      string `json:"preparation,omitempty"` // "statement" / "call"
	SanitizedQuery   string `json:"sanitized_query,omitempty"`
}

func makeSQL(attributes map[string]string) (map[string]string, *SQLData) {
	var (
		filtered    = make(map[string]string)
		sqlData     SQLData
		dbURL       string
		dbType      string
		dbInstance  string
		dbStatement string
		dbUser      string
	)

	for key, value := range attributes {
		switch key {
		case semconventions.AttributeDBURL:
			dbURL = value
		case semconventions.AttributeDBType:
			dbType = value
		case semconventions.AttributeDBInstance:
			dbInstance = value
		case semconventions.AttributeDBStatement:
			dbStatement = value
		case semconventions.AttributeDBUser:
			dbUser = value
		default:
			filtered[key] = value
		}
	}

	if dbType != "sql" {
		// Either no DB attributes or this is not an SQL DB.
		return attributes, nil
	}

	if dbURL == "" {
		dbURL = "localhost"
	}
	url := dbURL + "/" + dbInstance
	sqlData = SQLData{
		URL:            url,
		DatabaseType:   dbType,
		User:           dbUser,
		SanitizedQuery: dbStatement,
	}
	return filtered, &sqlData
}
