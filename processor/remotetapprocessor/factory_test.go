// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.EqualValues(t, "remotetap", factory.Type())
	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config)
}
