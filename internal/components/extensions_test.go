// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package components

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/ballastextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
)

func TestDefaultExtensions(t *testing.T) {
	allFactories, err := Components()
	require.NoError(t, err)

	extFactories := allFactories.Extensions
	endpoint := testutil.GetAvailableLocalAddress(t)

	tests := []struct {
		extension   config.Type
		getConfigFn getExtensionConfigFn
	}{
		{
			extension: "health_check",
			getConfigFn: func() config.Extension {
				cfg := extFactories["health_check"].CreateDefaultConfig().(*healthcheckextension.Config)
				cfg.TCPAddr.Endpoint = endpoint
				return cfg
			},
		},
		{
			extension: "pprof",
			getConfigFn: func() config.Extension {
				cfg := extFactories["pprof"].CreateDefaultConfig().(*pprofextension.Config)
				cfg.TCPAddr.Endpoint = endpoint
				return cfg
			},
		},
		{
			extension: "zpages",
			getConfigFn: func() config.Extension {
				cfg := extFactories["zpages"].CreateDefaultConfig().(*zpagesextension.Config)
				cfg.TCPAddr.Endpoint = endpoint
				return cfg
			},
		},
		{
			extension: "bearertokenauth",
			getConfigFn: func() config.Extension {
				cfg := extFactories["bearertokenauth"].CreateDefaultConfig().(*bearertokenauthextension.Config)
				cfg.BearerToken = "sometoken"
				return cfg
			},
		},
		{
			extension: "memory_ballast",
			getConfigFn: func() config.Extension {
				cfg := extFactories["memory_ballast"].CreateDefaultConfig().(*ballastextension.Config)
				return cfg
			},
		},
		{
			extension: "asapclient",
			getConfigFn: func() config.Extension {
				cfg := extFactories["asapclient"].CreateDefaultConfig().(*asapauthextension.Config)
				cfg.KeyID = "test_issuer/test_kid"
				cfg.Issuer = "test_issuer"
				cfg.Audience = []string{"some_service"}
				cfg.TTL = 10 * time.Second
				// Valid PEM data required for successful initialisation. Key not actually used anywhere.
				cfg.PrivateKey = "data:application/pkcs8;kid=test;base64,MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgE" +
					"AAkEA0ZPr5JeyVDoB8RyZqQsx6qUD+9gMFg1/0hgdAvmytWBMXQJYdwkK2dFJwwZcWJVhJGcOJBDfB/8tcbdJd34KZQIDAQ" +
					"ABAkBZD20tJTHJDSWKGsdJyNIbjqhUu4jXTkFFPK4Hd6jz3gV3fFvGnaolsD5Bt50dTXAiSCpFNSb9M9GY6XUAAdlBAiEA6" +
					"MccfdZRfVapxKtAZbjXuAgMvnPtTvkVmwvhWLT5Wy0CIQDmfE8Et/pou0Jl6eM0eniT8/8oRzBWgy9ejDGfj86PGQIgWePq" +
					"IL4OofRBgu0O5TlINI0HPtTNo12U9lbUIslgMdECICXT2RQpLcvqj+cyD7wZLZj6vrHZnTFVrnyR/cL2UyxhAiBswe/MCcD" +
					"7T7J4QkNrCG+ceQGypc7LsxlIxQuKh5GWYA=="
				return cfg
			},
		},
		{
			extension: "ecs_task_observer",
			getConfigFn: func() config.Extension {
				cfg := extFactories["ecs_task_observer"].CreateDefaultConfig().(*ecstaskobserver.Config)
				cfg.Endpoint = "http://localhost"
				return cfg
			},
		},
	}

	// * The OIDC Auth extension requires an OIDC server to get the config from, and we don't want to spawn one here for this test.
	assert.Equal(t, len(tests)+8 /* not tested */, len(extFactories))

	for _, tt := range tests {
		t.Run(string(tt.extension), func(t *testing.T) {
			factory, ok := extFactories[tt.extension]
			require.True(t, ok)
			assert.Equal(t, tt.extension, factory.Type())
			assert.Equal(t, config.NewComponentID(tt.extension), factory.CreateDefaultConfig().ID())

			verifyExtensionLifecycle(t, factory, tt.getConfigFn)
		})
	}
}

// getExtensionConfigFn is used customize the configuration passed to the verification.
// This is used to change ports or provide values required but not provided by the
// default configuration.
type getExtensionConfigFn func() config.Extension

// verifyExtensionLifecycle is used to test if an extension type can handle the typical
// lifecycle of a component. The getConfigFn parameter only need to be specified if
// the test can't be done with the default configuration for the component.
func verifyExtensionLifecycle(t *testing.T, factory component.ExtensionFactory, getConfigFn getExtensionConfigFn) {
	ctx := context.Background()
	host := newAssertNoErrorHost(t)
	extCreateSet := componenttest.NewNopExtensionCreateSettings()

	if getConfigFn == nil {
		getConfigFn = factory.CreateDefaultConfig
	}

	firstExt, err := factory.CreateExtension(ctx, extCreateSet, getConfigFn())
	require.NoError(t, err)
	require.NoError(t, firstExt.Start(ctx, host))
	require.NoError(t, firstExt.Shutdown(ctx))

	secondExt, err := factory.CreateExtension(ctx, extCreateSet, getConfigFn())
	require.NoError(t, err)
	require.NoError(t, secondExt.Start(ctx, host))
	require.NoError(t, secondExt.Shutdown(ctx))
}

// assertNoErrorHost implements a component.Host that asserts that there were no errors.
type assertNoErrorHost struct {
	component.Host
	*testing.T
}

var _ component.Host = (*assertNoErrorHost)(nil)

// newAssertNoErrorHost returns a new instance of assertNoErrorHost.
func newAssertNoErrorHost(t *testing.T) component.Host {
	return &assertNoErrorHost{
		componenttest.NewNopHost(),
		t,
	}
}

func (aneh *assertNoErrorHost) ReportFatalError(err error) {
	assert.NoError(aneh, err)
}
