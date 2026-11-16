// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:debug fips140=only

package fips140test

import (
	"crypto/fips140"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/metadata"
)

func newServer(t *testing.T, inline string) extension.Extension {
	factory := basicauthextension.NewFactory()
	cfg := factory.CreateDefaultConfig().(*basicauthextension.Config)
	cfg.Htpasswd = &basicauthextension.HtpasswdSettings{Inline: inline}
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	return ext
}

func basic(user, password string) map[string][]string {
	return map[string][]string{"authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))}}
}

func TestPlainAndBcryptUnderFIPS140Only(t *testing.T) {
	require.True(t, fips140.Enabled(), "//go:debug fips140=only not applied")

	srv := newServer(t, "plain:secret\nbcrypt:$2a$06$m0CrhHm10qJ3lXRY.5zDGO3rS2KdeeWLuGmsfGlMfOxih58VYVfxe\n")
	require.NoError(t, srv.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, srv.Shutdown(t.Context())) })

	_, err := srv.(extensionauth.Server).Authenticate(t.Context(), basic("plain", "secret"))
	require.NoError(t, err)
	_, err = srv.(extensionauth.Server).Authenticate(t.Context(), basic("bcrypt", "a"))
	require.NoError(t, err)
	_, err = srv.(extensionauth.Server).Authenticate(t.Context(), basic("plain", "wrong"))
	require.Error(t, err)
}

func TestMD5AndSHA1EntriesRejectedUnderFIPS140Only(t *testing.T) {
	srv := newServer(t, "md5user:$apr1$FVVioVP7$ZdIWPG1p4E/ErujO7kA2n0\nshauser:{SHA}vFznddje0Ht4+pmO0FaxwrUKN/M=\n")
	err := srv.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.ErrorContains(t, err, `"md5user"`)
	assert.ErrorContains(t, err, `"shauser"`)
	assert.ErrorContains(t, err, "FIPS 140-3")
}
