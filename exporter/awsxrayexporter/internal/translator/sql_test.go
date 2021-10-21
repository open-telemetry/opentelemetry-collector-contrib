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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]pdata.AttributeValue)
	attributes[conventions.AttributeDBSystem] = pdata.NewAttributeValueString("mysql")
	attributes[conventions.AttributeDBName] = pdata.NewAttributeValueString("customers")
	attributes[conventions.AttributeDBStatement] = pdata.NewAttributeValueString("SELECT * FROM user WHERE user_id = ?")
	attributes[conventions.AttributeDBUser] = pdata.NewAttributeValueString("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pdata.NewAttributeValueString("mysql://db.example.com:3306")
	attributes[conventions.AttributeNetPeerName] = pdata.NewAttributeValueString("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pdata.NewAttributeValueString("3306")

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
	attributes := make(map[string]pdata.AttributeValue)
	attributes[conventions.AttributeDBSystem] = pdata.NewAttributeValueString("redis")
	attributes[conventions.AttributeDBName] = pdata.NewAttributeValueString("0")
	attributes[conventions.AttributeDBStatement] = pdata.NewAttributeValueString("SET key value")
	attributes[conventions.AttributeDBUser] = pdata.NewAttributeValueString("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pdata.NewAttributeValueString("redis://db.example.com:3306")
	attributes[conventions.AttributeNetPeerName] = pdata.NewAttributeValueString("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pdata.NewAttributeValueString("3306")

	filtered, sqlData := makeSQL(attributes)
	assert.Nil(t, sqlData)
	assert.NotNil(t, filtered)
}

func TestClientSpanWithoutDBurlAttribute(t *testing.T) {
	attributes := make(map[string]pdata.AttributeValue)
	attributes[conventions.AttributeDBSystem] = pdata.NewAttributeValueString("postgresql")
	attributes[conventions.AttributeDBName] = pdata.NewAttributeValueString("customers")
	attributes[conventions.AttributeDBStatement] = pdata.NewAttributeValueString("SELECT * FROM user WHERE user_id = ?")
	attributes[conventions.AttributeDBUser] = pdata.NewAttributeValueString("readonly_user")
	attributes[conventions.AttributeDBConnectionString] = pdata.NewAttributeValueString("")
	attributes[conventions.AttributeNetPeerName] = pdata.NewAttributeValueString("db.example.com")
	attributes[conventions.AttributeNetPeerPort] = pdata.NewAttributeValueString("3306")
	filtered, sqlData := makeSQL(attributes)
	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	assert.Equal(t, "localhost/customers", *sqlData.URL)
}
