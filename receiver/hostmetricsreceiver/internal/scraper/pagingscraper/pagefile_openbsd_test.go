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

// +build freebsd openbsd

package pagingscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validFile = `Device       1K-blocks      Used	Avail	Capacity	Priority
/dev/wd0b    655025          1234	653791	1%	0
`

const invalidFile = `Device       1024-blocks      Used	Avail	Capacity	Priority
/dev/wd0b    655025          1234	653791	1%	0
`

func TestParseSwapctlOutput_Valid(t *testing.T) {
	assert := assert.New(t)
	stats, err := parseSwapctlOutput(validFile)
	assert.NoError(err)

	assert.Equal(*stats[0], pageFileStats{
		deviceName: "/dev/wd0b",
		usedBytes:  1234 * 1024,
		freeBytes:  653791 * 1024,
	})
}

func TestParseSwapctlOutput_Invalid(t *testing.T) {
	_, err := parseSwapctlOutput(invalidFile)
	assert.Error(t, err)
}

func TestParseSwapctlOutput_Empty(t *testing.T) {
	_, err := parseSwapctlOutput("")
	assert.Error(t, err)
}
