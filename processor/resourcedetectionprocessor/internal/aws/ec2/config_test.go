// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/stretchr/testify/assert"
)

// Keep local defaults aligned with the SDK and flag changes during SDK upgrades.
// Maintenance steps: ../../../README.md#maintaining-the-ec2-retry-defaults.
func TestDefaultRetrySettingsMatchSDK(t *testing.T) {
	const maintenancePath = "processor/resourcedetectionprocessor/README.md#maintaining-the-ec2-retry-defaults"
	const maintenanceURL = "https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor#maintaining-the-ec2-retry-defaults"
	const instructions = "AWS SDK retry defaults differ from the local EC2 config; follow the maintenance steps in %s (%s)"
	cfg := CreateDefaultConfig()
	assert.Equal(t, retry.DefaultMaxAttempts, cfg.MaxAttempts, instructions, maintenancePath, maintenanceURL)
	assert.Equal(t, retry.DefaultMaxBackoff, cfg.MaxBackoff, instructions, maintenancePath, maintenanceURL)
}
