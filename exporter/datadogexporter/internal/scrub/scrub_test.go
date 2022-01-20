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

package scrub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

func TestScrubber(t *testing.T) {
	tests := []struct {
		name     string
		original string
		expected string
	}{
		{
			name:     "no scrub",
			original: "This is an error",
			expected: "This is an error",
		},
		{
			name:     "no scrub (container id)",
			original: "container id: b32bd6f9b73ba7ccb64953a04b82b48e29dfafab65fd57ca01d3b94a0e024885",
			expected: "container id: b32bd6f9b73ba7ccb64953a04b82b48e29dfafab65fd57ca01d3b94a0e024885",
		},
		{
			name:     "api key (as parameter)",
			original: "Post \"https://api.datadoghq.com/api/v1/series?api_key=aaaaaaaaaaaaaaaaaaaaaaaaaaaabbbb&application_key=\": EOF",
			expected: "Post \"https://api.datadoghq.com/api/v1/series?api_key=***************************abbbb&application_key=\": EOF",
		},
		{
			name:     "api key",
			original: "aaaaaaaaaaaaaaaaaaaaaaaaaaaabbbb something aaaaaaaaaaaaaaaaaaaaaaaaaaaabbbb",
			expected: "***************************abbbb something ***************************abbbb",
		},
		{
			name:     "app key (as parameter)",
			original: "Failed to connect to http://something?app_key=reallylong40characterssecretkeygoeshere",
			expected: "Failed to connect to http://something?app_key=***********************************shere",
		},
		{
			name:     "app key",
			original: "should scrub: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABBBB",
			expected: "should scrub: ***********************************ABBBB",
		},
	}

	scrubber := NewScrubber()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.EqualError(t, scrubber.Scrub(errors.New(test.original)), test.expected)
		})
	}
}

func TestPermanentErrorScrub(t *testing.T) {
	err := consumererror.NewPermanent(errors.New("this is an error with an app key AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABBBB"))
	scrubber := NewScrubber()
	err = scrubber.Scrub(err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.EqualError(t, err, "Permanent error: this is an error with an app key ***********************************ABBBB")
}
