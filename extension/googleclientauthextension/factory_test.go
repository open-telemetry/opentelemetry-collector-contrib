// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googleclientauthextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/metadata"
)

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("google_client_auth"), factory.Type())

	for _, typ := range []component.Type{metadata.Type, component.MustNewType("googleclientauth")} {
		t.Run(typ.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig()
			comp, err := factory.Create(t.Context(), extensiontest.NewNopSettings(typ), cfg)
			require.NoError(t, err)
			require.NotNil(t, comp)
			require.NoError(t, comp.Shutdown(t.Context()))
		})
	}
}
