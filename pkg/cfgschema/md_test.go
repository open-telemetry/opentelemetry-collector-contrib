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

package cfgschema

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestGenerateMDFiles(t *testing.T) {
	err := generateMDFiles(otelcol.Factories{Receivers: map[component.Type]receiver.Factory{
		"nop": receivertest.NewNopFactory(),
	}}, func(v reflect.Value) (map[string]string, error) {
		return nil, nil
	}, func(v reflect.Value, data []byte) error {
		assert.True(t, strings.HasPrefix(string(data), `# Nop Receiver Reference`))
		return nil
	})
	require.NoError(t, err)
}
