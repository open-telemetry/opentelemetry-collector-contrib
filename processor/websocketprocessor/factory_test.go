// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.EqualValues(t, "websocket", factory.Type())
	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config)
}
