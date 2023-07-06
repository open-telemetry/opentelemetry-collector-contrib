// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestNewJSONUnmarshaler(t *testing.T) {
	t.Parallel()
	um := newJSONLogsUnmarshaler()
	assert.Equal(t, "json", um.Encoding())
}

func TestPlogReturnType(t *testing.T) {
	t.Parallel()
	um := newJSONLogsUnmarshaler()
	json := `{"example": "example valid json to test that the unmarshaler is correctly returning a plog value"}`

	unmarshaledJSON, err := um.Unmarshal([]byte(json))

	assert.NoError(t, err)
	assert.Nil(t, err)

	var expectedType plog.Logs
	assert.IsType(t, expectedType, unmarshaledJSON)
}
