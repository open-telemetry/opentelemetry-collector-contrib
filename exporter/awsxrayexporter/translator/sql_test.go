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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
)

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]string)
	attributes[semconventions.AttributeComponent] = "db"
	attributes[semconventions.AttributeDBType] = "sql"
	attributes[semconventions.AttributeDBInstance] = "customers"
	attributes[semconventions.AttributeDBStatement] = "SELECT * FROM user WHERE user_id = ?"
	attributes[semconventions.AttributeDBUser] = "readonly_user"
	attributes[semconventions.AttributeDBURL] = "mysql://db.example.com:3306"
	attributes[semconventions.AttributeNetPeerName] = "db.example.com"
	attributes[semconventions.AttributeNetPeerPort] = "3306"

	filtered, sqlData := makeSQL(attributes)

	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)
	w := testWriters.borrow()
	if err := w.Encode(sqlData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	testWriters.release(w)
	assert.True(t, strings.Contains(jsonStr, "mysql://db.example.com:3306/customers"))
}

func TestClientSpanWithNonSQLDatabase(t *testing.T) {
	attributes := make(map[string]string)
	attributes[semconventions.AttributeComponent] = "db"
	attributes[semconventions.AttributeDBType] = "redis"
	attributes[semconventions.AttributeDBInstance] = "0"
	attributes[semconventions.AttributeDBStatement] = "SET key value"
	attributes[semconventions.AttributeDBUser] = "readonly_user"
	attributes[semconventions.AttributeDBURL] = "redis://db.example.com:3306"
	attributes[semconventions.AttributeNetPeerName] = "db.example.com"
	attributes[semconventions.AttributeNetPeerPort] = "3306"

	filtered, sqlData := makeSQL(attributes)
	assert.Nil(t, sqlData)
	assert.NotNil(t, filtered)
}
