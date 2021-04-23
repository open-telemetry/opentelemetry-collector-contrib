// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ensure that truncation helperr function truncates strings as expected
// and accounts for the limit and multi byte ending characters
// from https://github.com/DataDog/datadog-agent/blob/140a4ee164261ef2245340c50371ba989fbeb038/pkg/trace/traceutil/truncate_test.go#L15
func TestTruncateUTF8Strings(t *testing.T) {
	assert.Equal(t, "", TruncateUTF8("", 5))
	assert.Equal(t, "télé", TruncateUTF8("télé", 5))
	assert.Equal(t, "t", TruncateUTF8("télé", 2))
	assert.Equal(t, "éé", TruncateUTF8("ééééé", 5))
	assert.Equal(t, "ééééé", TruncateUTF8("ééééé", 18))
	assert.Equal(t, "ééééé", TruncateUTF8("ééééé", 10))
	assert.Equal(t, "ééé", TruncateUTF8("ééééé", 6))
}
