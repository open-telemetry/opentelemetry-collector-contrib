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

// nolint:errcheck
package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestLumigoAuth_NoHeader(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{})
	require.NoError(t, err)
	_, err = ext.Authenticate(context.Background(), map[string][]string{})
	assert.Equal(t, errNoAuth, err)
}

func TestLumigoAuth_InvalidPrefix(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{})
	require.NoError(t, err)
	_, err = ext.Authenticate(context.Background(), map[string][]string{"authorization": {"Bearer token"}})
	assert.Equal(t, errInvalidSchemePrefix, err)
}

func TestLumigoAuth_SupportedHeaders(t *testing.T) {
	ext, err := newServerAuthExtension(&Config{})
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	for _, k := range []string{
		"Authorization",
		"authorization",
		"aUtHoRiZaTiOn",
	} {
		_, err = ext.Authenticate(context.Background(), map[string][]string{k: {"LumigoToken 1234567"}})
		assert.NoError(t, err)
	}
}
