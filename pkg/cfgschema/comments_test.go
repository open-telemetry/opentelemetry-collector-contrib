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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentsForStruct(t *testing.T) {
	cr := commentReader{dirResolver{
		srcRoot:    ".",
		moduleName: "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/cfgschema",
	}}
	comments, err := cr.commentsForStruct(reflect.ValueOf(testConfig{}))
	require.NoError(t, err)
	assert.Equal(t, "Name is a required field\n", comments["Name"])
}

type testConfig struct {
	// Name is a required field
	Name string `mapstructure:"name"`
}
