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

// +build freebsd

package pagingscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validFile = `Device:       1kB-blocks      Used:
/dev/gpt/swapfs    1048576          1234
/dev/md0         1048576          666
`

const invalidFile = `Device:       512-blocks      Used:
/dev/gpt/swapfs    1048576          1234
/dev/md0         1048576          666
`

func TestParseSwapctlOutput_Valid(t *testing.T) {
	assert := assert.New(t)
	stats, err := parseSwapctlOutput(validFile)
	assert.NoError(err)

	assert.Equal(*stats[0], pageFileStats{
		deviceName: "/dev/gpt/swapfs",
		usedBytes:  1263616,
		freeBytes:  1072478208,
	})

	assert.Equal(*stats[1], pageFileStats{
		deviceName: "/dev/md0",
		usedBytes:  681984,
		freeBytes:  1073059840,
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
