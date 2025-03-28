// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAzureAuthenticator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ext := newAzureAuthenticator(cfg, zap.NewNop())
	require.NotNil(t, ext)
}
