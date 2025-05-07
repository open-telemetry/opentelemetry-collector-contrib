// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
)

func TestBearerToken(t *testing.T) {
	// prepare
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.Create(context.Background(), extensiontest.NewNopSettings(baFactory.Type()), baCfg)
	require.NoError(t, err)

	baComponentName := component.MustNewIDWithName("bearertokenauth", "fb02")

	host := &mockHost{
		Host: componenttest.NewNopHost(),
		extensions: map[component.ID]component.Component{
			baComponentName: baExt,
		},
	}

	cfgAuth := configauth.Config{
		AuthenticatorID: baComponentName,
	}

	// test
	token, err := RetrieveBearerToken(cfgAuth, host.GetExtensions())

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "the-token", token)
}

type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
