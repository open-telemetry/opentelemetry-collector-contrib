// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package activedirectorydsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()

	fact := NewFactory()
	require.NotNil(t, fact)
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	conf := createDefaultConfig()
	require.NotNil(t, conf)
}
