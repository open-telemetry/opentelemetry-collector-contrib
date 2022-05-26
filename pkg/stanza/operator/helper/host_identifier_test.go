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

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func MockHostIdentifierConfig(includeIP, includeHostname bool, ip, hostname string) HostIdentifierConfig {
	return HostIdentifierConfig{
		IncludeIP:       includeIP,
		IncludeHostname: includeHostname,
		getIP:           func() (string, error) { return ip, nil },
		getHostname:     func() (string, error) { return hostname, nil },
	}
}

func TestHostAttributer(t *testing.T) {
	cases := []struct {
		name             string
		config           HostIdentifierConfig
		expectedResource map[string]interface{}
	}{
		{
			"HostnameAndIP",
			MockHostIdentifierConfig(true, true, "ip", "hostname"),
			map[string]interface{}{
				"host.name": "hostname",
				"host.ip":   "ip",
			},
		},
		{
			"HostnameNoIP",
			MockHostIdentifierConfig(false, true, "ip", "hostname"),
			map[string]interface{}{
				"host.name": "hostname",
			},
		},
		{
			"IPNoHostname",
			MockHostIdentifierConfig(true, false, "ip", "hostname"),
			map[string]interface{}{
				"host.ip": "ip",
			},
		},
		{
			"NoHostnameNoIP",
			MockHostIdentifierConfig(false, false, "", "test"),
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identifier, err := tc.config.Build()
			require.NoError(t, err)

			e := entry.New()
			identifier.Identify(e)
			require.Equal(t, tc.expectedResource, e.Resource)
		})
	}
}
