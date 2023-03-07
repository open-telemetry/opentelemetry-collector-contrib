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

package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateDefaultConfig(t *testing.T) {
	expected := &Config{
		Token: "",
		Type:  Server,
	}
	actual := createDefaultConfig()
	assert.Equal(t, expected, createDefaultConfig())
	assert.NoError(t, componenttest.CheckConfigStruct(actual))
}

func TestClientAuth(t *testing.T) {
	cfg := &Config{
		Token: "",
		Type:  Client,
	}
	extension, err := createExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	assert.IsType(t, auth.NewClient(), extension)
	assert.NoError(t, err)
}

func TestServerAuth(t *testing.T) {
	cfg := &Config{
		Token: "",
		Type:  Server,
	}
	extension, err := createExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	assert.IsType(t, auth.NewServer(), extension)
	assert.NoError(t, err)
}
