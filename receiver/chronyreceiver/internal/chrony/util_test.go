// Copyright The OpenTelemetry Authors
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

package chrony

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitNetworkEndpoint(t *testing.T) {
	t.Parallel()

	path := t.TempDir()

	for _, tc := range []struct {
		scenario string
		in       string

		network, endpoint string
		err               error
	}{
		{
			scenario: "A valid UDP network",
			in:       "udp://localhost:323",
			network:  "udp",
			endpoint: "localhost:323",
			err:      nil,
		},
		{
			scenario: "Invalid UDP network (missing hostname)",
			in:       "udp://:323",
			network:  "",
			endpoint: "",
			err:      ErrInvalidNetwork,
		},
		{
			scenario: "Invalid UDP Network (missing port)",
			in:       "udp://localhost",
			network:  "",
			endpoint: "",
			err:      ErrInvalidNetwork,
		},
		{
			scenario: "A valid UNIX network",
			in:       fmt.Sprintf("unix://%s", path),
			network:  "unixgram",
			endpoint: path,
			err:      nil,
		},
		{
			scenario: "Invalid unix socket (not valid path)",
			in:       "unix:///path/does/not/exist",
			network:  "",
			endpoint: "",
			err:      os.ErrNotExist,
		},
		{
			scenario: "Invalid network",
			in:       "tcp://localhost:323",
			network:  "",
			endpoint: "",
			err:      ErrInvalidNetwork,
		},
		{
			scenario: "No input provided",
			in:       "",
			network:  "",
			endpoint: "",
			err:      ErrInvalidNetwork,
		},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			network, endpoint, err := SplitNetworkEndpoint(tc.in)

			assert.Equal(t, tc.network, network, "Must match the expected network")
			assert.Equal(t, tc.endpoint, endpoint, "Must match the expected endpoint")
			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
		})
	}
}
