// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

type dbAuthResolverHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h dbAuthResolverHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type dbAuthResolverExtension struct {
	component.Component
	dbauth.Provider
}

func TestDBAuthResolver(t *testing.T) {
	t.Parallel()
	id := component.MustNewID("azure_auth")
	provider := &dbAuthResolverExtension{
		Provider: dbAuthResolverProvider{},
	}
	resolver := newDBAuthResolver(configdbauth.ID(id))

	err := resolver.resolve(dbAuthResolverHost{
		extensions: map[component.ID]component.Component{id: provider},
	})
	require.NoError(t, err)
	require.Same(t, dbauth.Provider(provider), resolver.getProvider())
}

type dbAuthResolverProvider struct{}

func (dbAuthResolverProvider) GetCredential(context.Context, dbauth.Request) (*dbauth.Credential, error) {
	return &dbauth.Credential{Secret: "token"}, nil
}
