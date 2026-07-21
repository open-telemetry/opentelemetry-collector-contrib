// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerprovider

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)

	cfg := f.CreateDefaultConfig().(*Config)
	assert.Equal(t, 60*time.Minute, cfg.RefreshInterval)
	assert.Empty(t, cfg.SecretARN)
	assert.Empty(t, cfg.Region)
}
