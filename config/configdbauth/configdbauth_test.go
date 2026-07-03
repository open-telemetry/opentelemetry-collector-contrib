// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configdbauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

// fakeProvider is a credentials provider extension: it implements both
// component.Component (so it can live in the host extension map) and
// dbauth.Provider. It records the request and extension args GetCredential
// received.
type fakeProvider struct {
	cred    *dbauth.Credential
	gotReq  dbauth.Request
	gotArgs map[string]any
}

func (*fakeProvider) Start(context.Context, component.Host) error { return nil }
func (*fakeProvider) Shutdown(context.Context) error              { return nil }

func (f *fakeProvider) GetCredential(_ context.Context, req dbauth.Request, extensionArgs map[string]any) (*dbauth.Credential, error) {
	f.gotReq = req
	f.gotArgs = extensionArgs
	return f.cred, nil
}

// notAProvider is an extension that does not implement dbauth.Provider.
type notAProvider struct{}

func (notAProvider) Start(context.Context, component.Host) error { return nil }
func (notAProvider) Shutdown(context.Context) error              { return nil }

// extMap builds a host extension map from one extension under the given ID.
func extMap(id component.ID, ext component.Component) map[component.ID]component.Component {
	return map[component.ID]component.Component{id: ext}
}

// providerConfig builds a Config whose single db_auth key is the given provider
// ID with the given inline override value.
func providerConfig(id string, args map[string]any) Config {
	return Config{ProviderConfigs: map[string]any{id: args}}
}

func TestConfig_IsEmpty(t *testing.T) {
	assert.True(t, Config{}.IsEmpty())
	assert.False(t, providerConfig("aws_iam", nil).IsEmpty())
}

func TestConfig_Validate(t *testing.T) {
	assert.NoError(t, Config{}.Validate(), "zero providers is opt-out, allowed")
	assert.NoError(t, providerConfig("aws_iam", nil).Validate(), "exactly one provider is allowed")

	two := Config{ProviderConfigs: map[string]any{"aws_iam": nil, "vault": nil}}
	require.ErrorIs(t, two.Validate(), errMultipleProviders)
}

func TestConfig_GetProvider_MatchesExtensionByID(t *testing.T) {
	id := component.MustNewID("aws_iam")
	f := &fakeProvider{cred: &dbauth.Credential{Secret: "tok"}}

	cfg := providerConfig("aws_iam", nil)
	p, args, err := cfg.GetProvider(extMap(id, f))
	require.NoError(t, err)
	require.NotNil(t, p)

	// The resolved provider is the declared extension itself.
	assert.Same(t, f, p)
	assert.Nil(t, args, "no inline body means no override")
}

func TestConfig_GetProvider_ReturnsInlineOverride(t *testing.T) {
	id := component.MustNewID("aws_iam")
	f := &fakeProvider{cred: &dbauth.Credential{Secret: "tok"}}

	override := map[string]any{"region": "us-east-1"}
	cfg := providerConfig("aws_iam", override)
	p, args, err := cfg.GetProvider(extMap(id, f))
	require.NoError(t, err)
	assert.Same(t, f, p)
	assert.Equal(t, override, args, "the inline value under the provider ID is returned as extensionArgs")
}

func TestConfig_GetProvider_EmptyErrors(t *testing.T) {
	_, _, err := Config{}.GetProvider(extMap(component.MustNewID("aws_iam"), &fakeProvider{}))
	require.ErrorIs(t, err, errNoCredentials)
}

func TestConfig_GetProvider_MultipleProvidersErrors(t *testing.T) {
	cfg := Config{ProviderConfigs: map[string]any{"aws_iam": nil, "vault": nil}}
	_, _, err := cfg.GetProvider(extMap(component.MustNewID("aws_iam"), &fakeProvider{}))
	require.ErrorIs(t, err, errMultipleProviders)
}

func TestConfig_GetProvider_NoMatchingExtension(t *testing.T) {
	cfg := providerConfig("vault", nil)
	_, _, err := cfg.GetProvider(extMap(component.MustNewID("aws_iam"), &fakeProvider{}))
	require.ErrorIs(t, err, errNoExtension)
}

func TestConfig_GetProvider_ExtensionNotAProvider(t *testing.T) {
	id := component.MustNewID("aws_iam")
	cfg := providerConfig("aws_iam", nil)
	_, _, err := cfg.GetProvider(extMap(id, notAProvider{}))
	require.ErrorIs(t, err, errNotProvider)
}

func TestConfig_GetProvider_NamedInstance(t *testing.T) {
	// A provider extension may be declared with a name (aws_iam/primary); the
	// consumer references it by the full ID.
	id := component.MustNewIDWithName("aws_iam", "primary")
	f := &fakeProvider{cred: &dbauth.Credential{Secret: "tok"}}

	cfg := providerConfig("aws_iam/primary", nil)
	p, _, err := cfg.GetProvider(extMap(id, f))
	require.NoError(t, err)
	assert.Same(t, f, p)
}
