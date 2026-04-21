// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestJSONLogsUnmarshaler(t *testing.T) {
	var u plog.Unmarshaler = JSONLogsUnmarshaler{}
	json := `{"example": "example valid json to test that the unmarshaler is correctly returning a plog value"}`

	unmarshaledJSON, err := u.UnmarshalLogs([]byte(json))

	assert.NoError(t, err)
	assert.NoError(t, err)

	var expectedType plog.Logs
	assert.IsType(t, expectedType, unmarshaledJSON)
}
