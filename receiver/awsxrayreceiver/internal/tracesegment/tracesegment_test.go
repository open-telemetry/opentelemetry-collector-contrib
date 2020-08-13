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

package tracesegment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceSegmentHeaderIsValid(t *testing.T) {
	header := Header{
		Format:  "json",
		Version: 1,
	}

	valid := header.IsValid()

	assert.True(t, valid)
}

func TestTraceSegmentHeaderIsValidCaseInsensitive(t *testing.T) {
	header := Header{
		Format:  "jSoN",
		Version: 1,
	}

	valid := header.IsValid()

	assert.True(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongVersion(t *testing.T) {
	header := Header{
		Format:  "json",
		Version: 2,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongFormat(t *testing.T) {
	header := Header{
		Format:  "xml",
		Version: 1,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongFormatVersion(t *testing.T) {
	header := Header{
		Format:  "xml",
		Version: 2,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}
