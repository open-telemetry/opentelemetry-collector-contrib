// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetelemetrypolicyextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/filetelemetrypolicyextension/internal/metadata"
)

func TestExtensionLifecycle(t *testing.T) {
	cfg := &Config{}
	ext := newExtension(cfg, extensiontest.NewNopSettings(metadata.Type))
	require.NotNil(t, ext)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(t.Context()))
}
