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

package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestCanDetermineIfAttributeIsSet(t *testing.T) {
	attrMap := pcommon.NewMap()
	attrMap.PutStr("foo", "bar")
	attrMap.PutStr("fizz", "buzz")

	assert.Equal(t, false, containsAttributes(attrMap, "bingo"))
	assert.Equal(t, false, containsAttributes(attrMap, "bingo", "buzz"))

	assert.Equal(t, true, containsAttributes(attrMap, "foo"))
	assert.Equal(t, true, containsAttributes(attrMap, "foo", "fizz"))
}
