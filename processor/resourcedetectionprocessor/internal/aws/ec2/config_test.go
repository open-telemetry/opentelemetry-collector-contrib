// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/stretchr/testify/assert"
)

// Keep local defaults aligned with the selected SDK version. If an upgrade changes
// these values, review the change and update the constants in config.go if intended.
// SDK definitions: https://github.com/aws/aws-sdk-go-v2/blob/v1.47.1/aws/retry/standard.go#L26-L33.
func TestDefaultRetrySettingsMatchSDK(t *testing.T) {
	cfg := CreateDefaultConfig()
	assert.Equal(t, retry.DefaultMaxAttempts, cfg.MaxAttempts,
		"review the selected AWS SDK's retry.DefaultMaxAttempts and update defaultMaxAttempts in config.go if the change is intended")
	assert.Equal(t, retry.DefaultMaxBackoff, cfg.MaxBackoff,
		"review the selected AWS SDK's retry.DefaultMaxBackoff and update defaultMaxBackoff in config.go if the change is intended")
}
