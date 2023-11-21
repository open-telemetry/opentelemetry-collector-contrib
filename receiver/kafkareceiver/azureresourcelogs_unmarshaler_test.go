// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewAzureResourceLogsUnmarshaler(t *testing.T) {
	um := newAzureResourceLogsUnmarshaler("Test Version", zap.NewNop())
	assert.Equal(t, "azureresourcelogs", um.Encoding())
}
