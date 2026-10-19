// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/stretchr/testify/assert"
)

// Keep local defaults aligned with the SDK and flag changes during SDK upgrades.
func TestDefaultRetrySettingsMatchSDK(t *testing.T) {
	cfg := CreateDefaultConfig()
	assert.Equal(t, retry.DefaultMaxAttempts, cfg.MaxAttempts)
	assert.Equal(t, retry.DefaultMaxBackoff, cfg.MaxBackoff)
}
