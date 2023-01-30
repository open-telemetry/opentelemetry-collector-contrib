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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestGetAllCfgInfos(t *testing.T) {
	infos := getAllCfgInfos(otelcol.Factories{Receivers: map[component.Type]receiver.Factory{"": receivertest.NewNopFactory()}})
	assert.Len(t, infos, 1)
	ci := infos[0]
	assert.Equal(t, "receiver", ci.Group)
	assert.EqualValues(t, "nop", ci.Type)
}
