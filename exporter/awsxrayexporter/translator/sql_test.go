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
)

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]string)
	attributes[ComponentAttribute] = DbComponentType
	attributes[DbTypeAttribute] = "sql"
	attributes[DbInstanceAttribute] = "customers"
	attributes[DbStatementAttribute] = "SELECT * FROM user WHERE user_id = ?"
	attributes[DbUserAttribute] = "readonly_user"
	attributes[PeerAddressAttribute] = "mysql://db.example.com:3306"
	attributes[PeerHostAttribute] = "db.example.com"
	attributes[PeerPortAttribute] = "3306"

	filtered, sqlData := makeSQL(attributes)

	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)
	w := borrow()
	if err := w.Encode(sqlData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, "mysql://db.example.com:3306/customers"))
}

func TestClientSpanWithHttpComponentAttribute(t *testing.T) {
	attributes := make(map[string]string)
	attributes[ComponentAttribute] = HTTPComponentType
	attributes[DbTypeAttribute] = "sql"
	attributes[DbInstanceAttribute] = "customers"
	attributes[DbStatementAttribute] = "SELECT * FROM user WHERE user_id = ?"
	attributes[DbUserAttribute] = "readonly_user"
	attributes[PeerAddressAttribute] = "mysql://db.example.com:3306"
	attributes[PeerHostAttribute] = "db.example.com"
	attributes[PeerPortAttribute] = "3306"

	filtered, sqlData := makeSQL(attributes)

	assert.NotNil(t, filtered)
	assert.Nil(t, sqlData)
}
