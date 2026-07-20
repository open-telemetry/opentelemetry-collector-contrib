// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscredentialsproviderextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/awscredentialsproviderextension/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, metadata.Type, factory.Type())

	cfg := factory.CreateDefaultConfig()
	// The default config is intentionally not valid: the user must configure an
	// explicit credential source.
	require.ErrorIs(t, cfg.(*Config).Validate(), errNoCredentialSource)

	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
