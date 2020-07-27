// Copyright 2020, OpenTelemetry Authors
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

package transport

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/testutil"
)

func Test_Server_ListenAndServe(t *testing.T) {
	tests := []struct {
		name          string
		buildServerFn func(addr string) (Server, error)
		// TODO: build StatsD client for testing.
		// buildClientFn func(host string, port int) (*client.StatsD, error)
	}{
		{
			name: "udp",
			buildServerFn: func(addr string) (Server, error) {
				return NewUDPServer(addr)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := testutil.GetAvailableLocalAddress(t)
			srv, err := tt.buildServerFn(addr)
			require.NoError(t, err)
			require.NotNil(t, srv)
		})
	}
}
