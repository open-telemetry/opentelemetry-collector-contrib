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
// dbauth.Provider. It records the request GetCredential received.
type fakeProvider struct {
	cred   *dbauth.Credential
	gotReq dbauth.Request
}

func (*fakeProvider) Start(context.Context, component.Host) error { return nil }
func (*fakeProvider) Shutdown(context.Context) error              { return nil }

func (f *fakeProvider) GetCredential(_ context.Context, req dbauth.Request) (*dbauth.Credential, error) {
	f.gotReq = req
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

// providerID builds an ID referencing the given provider component ID.
func providerID(id string) ID {
	var pid ID
	if err := pid.UnmarshalText([]byte(id)); err != nil {
		panic(err)
	}
	return pid
}

func TestID_IsEmpty(t *testing.T) {
	assert.True(t, ID{}.IsEmpty())
	assert.False(t, providerID("aws_iam").IsEmpty())
}

func TestID_UnmarshalScalarID(t *testing.T) {
	// The db_auth value is a scalar component ID; confmap's text-unmarshaler decode
	// hook invokes UnmarshalText on the string, exactly as it does for a bare
	// component.ID field. Exercising UnmarshalText directly proves that wiring
	// without pulling confmap into this dependency-light module.
	var id ID
	require.NoError(t, id.UnmarshalText([]byte("aws_iam")))
	assert.Equal(t, component.MustNewID("aws_iam"), id.ComponentID())
}

func TestID_UnmarshalNamedInstance(t *testing.T) {
	var id ID
	require.NoError(t, id.UnmarshalText([]byte("aws_iam/primary")))
	assert.Equal(t, component.MustNewIDWithName("aws_iam", "primary"), id.ComponentID())
}

func TestID_MarshalText(t *testing.T) {
	got, err := providerID("aws_iam/primary").MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "aws_iam/primary", string(got))
}

func TestID_UnmarshalInvalidID(t *testing.T) {
	var id ID
	require.Error(t, id.UnmarshalText([]byte("Not A Valid ID")))
}

func TestID_GetProvider_MatchesExtensionByID(t *testing.T) {
	id := component.MustNewID("aws_iam")
	f := &fakeProvider{cred: &dbauth.Credential{Secret: "tok"}}

	p, err := providerID("aws_iam").GetProvider(extMap(id, f))
	require.NoError(t, err)
	require.NotNil(t, p)

	// The resolved provider is the declared extension itself.
	assert.Same(t, f, p)
}

func TestID_GetProvider_EmptyErrors(t *testing.T) {
	_, err := ID{}.GetProvider(extMap(component.MustNewID("aws_iam"), &fakeProvider{}))
	require.ErrorIs(t, err, errNoCredentials)
}

func TestID_GetProvider_NoMatchingExtension(t *testing.T) {
	_, err := providerID("vault").GetProvider(extMap(component.MustNewID("aws_iam"), &fakeProvider{}))
	require.ErrorIs(t, err, errNoExtension)
}

func TestID_GetProvider_ExtensionNotAProvider(t *testing.T) {
	id := component.MustNewID("aws_iam")
	_, err := providerID("aws_iam").GetProvider(extMap(id, notAProvider{}))
	require.ErrorIs(t, err, errNotProvider)
}

func TestID_GetProvider_NamedInstance(t *testing.T) {
	// A provider extension may be declared with a name (aws_iam/primary); the
	// consumer references it by the full ID.
	id := component.MustNewIDWithName("aws_iam", "primary")
	f := &fakeProvider{cred: &dbauth.Credential{Secret: "tok"}}

	p, err := providerID("aws_iam/primary").GetProvider(extMap(id, f))
	require.NoError(t, err)
	assert.Same(t, f, p)
}
