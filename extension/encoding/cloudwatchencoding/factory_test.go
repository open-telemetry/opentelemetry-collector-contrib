// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencoding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
