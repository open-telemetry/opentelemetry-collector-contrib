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

package otel2xray

const (
	DbTypeAttribute      = "db.type"
	DbInstanceAttribute  = "db.instance"
	DbStatementAttribute = "db.statement"
	DbUserAttribute      = "db.user"
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

func makeSql(attributes map[string]string) *SQLData {
	var (
		sqlData SQLData
	)
	componentType := attributes[ComponentAttribute]
	url := attributes[PeerAddressAttribute]
	if componentType == HttpComponentType || componentType == RpcComponentType || url == "" {
		return nil
	}
	url += "/" + attributes[DbInstanceAttribute]
	sqlData = SQLData{
		URL:            url,
		DatabaseType:   attributes[DbTypeAttribute],
		User:           attributes[DbUserAttribute],
		SanitizedQuery: attributes[DbStatementAttribute],
	}
	return &sqlData
}
