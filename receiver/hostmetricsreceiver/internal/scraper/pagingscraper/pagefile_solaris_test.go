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

// +build solaris

package pagingscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validFile = `swapfile                  dev  swaplo blocks   free
/dev/zvol/dsk/rpool/swap 256,1      16 1058800 1058800
/dev/dsk/c0t0d0s1   136,1      16 1638608 1600528`

const invalidFile = `swapfile                  dev  swaplo INVALID   free
/dev/zvol/dsk/rpool/swap 256,1      16 1058800 1058800
/dev/dsk/c0t0d0s1   136,1      16 1638608 1600528`

func TestParseSwapsCommandOutput_Valid(t *testing.T) {
	assert := assert.New(t)
	stats, err := parseSwapsCommandOutput(validFile)
	assert.NoError(err)

	assert.Equal(*stats[0], pageFileStats{
		deviceName: "/dev/zvol/dsk/rpool/swap",
		usedBytes:  0,
		freeBytes:  1058800 * 512,
	})

	assert.Equal(*stats[1], pageFileStats{
		deviceName: "/dev/dsk/c0t0d0s1",
		usedBytes:  38080 * 512,
		freeBytes:  1600528 * 512,
	})
}

func TestParseSwapsCommandOutput_Invalid(t *testing.T) {
	_, err := parseSwapsCommandOutput(invalidFile)
	assert.Error(t, err)
}

func TestParseSwapsCommandOutput_Empty(t *testing.T) {
	_, err := parseSwapsCommandOutput("")
	assert.Error(t, err)
}
