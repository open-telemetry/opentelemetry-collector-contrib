// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
