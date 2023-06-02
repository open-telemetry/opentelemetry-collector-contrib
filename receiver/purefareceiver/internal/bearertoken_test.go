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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
)

func TestBearerToken(t *testing.T) {
	// prepare
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), baCfg)
	require.NoError(t, err)

	baComponentName := component.NewIDWithName("bearertokenauth", "array01")

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			baComponentName: baExt,
		},
	}

	cfgAuth := configauth.Authentication{
		AuthenticatorID: baComponentName,
	}

	// test
	token, err := RetrieveBearerToken(cfgAuth, host.GetExtensions())

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "the-token", token)
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (h *mockHost) ReportFatalError(_ error) {}

func (h *mockHost) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	return nil
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func (h *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return nil
}
