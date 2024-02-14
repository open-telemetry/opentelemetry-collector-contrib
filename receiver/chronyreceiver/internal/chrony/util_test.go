// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
