// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONUnmarshaler(t *testing.T) {
	t.Parallel()
	um := newJSONLogsUnmarshaler()
	assert.Equal(t, "json", um.Encoding())
}
