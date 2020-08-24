// Copyright The OpenTelemetry Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSQLURL(t *testing.T) {
	raw := "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432/ebdb"
	url, dbName, err := splitSQLURL(raw)
	assert.NoError(t, err, "should succeed")
	assert.Equal(t,
		"jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432",
		url, "expected url to be the same")

	assert.Equal(t,
		"ebdb",
		dbName, "expected db name to be the same")
}
func TestSQLURLQueryParameter(t *testing.T) {
	raw := "jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432/ebdb?myInterceptor=foo"
	url, dbName, err := splitSQLURL(raw)
	assert.NoError(t, err, "should succeed")
	assert.Equal(t,
		"jdbc:postgresql://aawijb5u25wdoy.cpamxznpdoq8.us-west-2.rds.amazonaws.com:5432",
		url, "expected url to be the same")

	assert.Equal(t,
		"ebdb",
		dbName, "expected db name to be the same")
}
