// Copyright  OpenTelemetry Authors
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

package httpdreceiver

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Run("error path", func(t *testing.T) {
		cfg := NewFactory().CreateDefaultConfig().(*Config)
		cfg.Endpoint = "http://endpoint with space"
		require.Error(t, cfg.Validate())
	})

	t.Run("happy path", func(t *testing.T) {
		testCases := []struct {
			desc     string
			rawURL   string
			expected string
		}{
			{
				desc:     "default path",
				rawURL:   "",
				expected: "http://localhost:8080/server-status?auto",
			},
			{
				desc:     "only host(local)",
				rawURL:   "localhost",
				expected: "http://localhost:8080/server-status?auto",
			},
			{
				desc:     "only host",
				rawURL:   "127.0.0.1",
				expected: "http://127.0.0.1:8080/server-status?auto",
			},
			{
				desc:     "host(local) and port",
				rawURL:   "localhost:8080",
				expected: "http://localhost:8080/server-status?auto",
			},
			{
				desc:     "host and port",
				rawURL:   "127.0.0.1:8080",
				expected: "http://127.0.0.1:8080/server-status?auto",
			},
			{
				desc:     "full path",
				rawURL:   "http://localhost:8080/server-status?auto",
				expected: "http://localhost:8080/server-status?auto",
			},
			{
				desc:     "full path",
				rawURL:   "http://127.0.0.1:8080/server-status?auto",
				expected: "http://127.0.0.1:8080/server-status?auto",
			},
			{
				desc:     "unique host no port",
				rawURL:   "myAlias.Site",
				expected: "http://myAlias.Site:8080/server-status?auto",
			},
			{
				desc:     "unique host with port",
				rawURL:   "myAlias.Site:1234",
				expected: "http://myAlias.Site:1234/server-status?auto",
			},
			{
				desc:     "unique host with port with path",
				rawURL:   "myAlias.Site:1234/server-status?auto",
				expected: "http://myAlias.Site:1234/server-status?auto",
			},
			{
				desc:     "only port",
				rawURL:   ":8080",
				expected: "http://localhost:8080/server-status?auto",
			},
			{
				desc:     "limitation: double port",
				rawURL:   "1234",
				expected: "http://1234:8080/server-status?auto",
			},
			{
				desc:     "limitation: invalid ip",
				rawURL:   "500.0.0.0.1.1",
				expected: "http://500.0.0.0.1.1:8080/server-status?auto",
			},
			{
				desc:     "custom path",
				rawURL:   "http://localhost:8080/custom?auto",
				expected: "http://localhost:8080/custom?auto",
			},
		}
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Endpoint = tC.rawURL
				require.NoError(t, cfg.Validate())
				require.Equal(t, tC.expected, cfg.Endpoint)
			})
		}
	})
}

func TestMissingProtocol(t *testing.T) {
	testCases := []struct {
		desc     string
		proto    string
		expected bool
	}{
		{
			desc:     "http proto",
			proto:    "http://localhost",
			expected: false,
		},
		{
			desc:     "https proto",
			proto:    "https://localhost",
			expected: false,
		},
		{
			desc:     "HTTP caps",
			proto:    "HTTP://localhost",
			expected: false,
		},
		{
			desc:     "everything else",
			proto:    "ht",
			expected: true,
		},
		{
			desc:     "everything else",
			proto:    "localhost",
			expected: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			require.Equal(t, tC.expected, missingProtocol(tC.proto))
		})
	}
}

func TestValidateEndpointFormat(t *testing.T) {
	protocols := []string{"", "http://", "https://"}
	hosts := []string{"", "127.0.0.1", "localhost", "customhost.com"}
	ports := []string{"", ":8080", ":1234"}
	paths := []string{"", "/server-status", "/custom"}
	endpoints := []string{}
	validEndpoints := map[string]bool{
		"http://127.0.0.1:8080/server-status?auto":      true,
		"http://127.0.0.1:1234/server-status?auto":      true,
		"http://localhost:8080/server-status?auto":      true,
		"http://localhost:1234/server-status?auto":      true,
		"http://customhost.com:8080/server-status?auto": true,
		"http://customhost.com:1234/server-status?auto": true,
		"http://127.0.0.1:8080/custom?auto":             true,
		"http://127.0.0.1:1234/custom?auto":             true,
		"http://localhost:8080/custom?auto":             true,
		"http://localhost:1234/custom?auto":             true,
		"http://customhost.com:8080/custom?auto":        true,
		"http://customhost.com:1234/custom?auto":        true,
		// https
		"https://127.0.0.1:8080/server-status?auto":      true,
		"https://127.0.0.1:1234/server-status?auto":      true,
		"https://localhost:8080/server-status?auto":      true,
		"https://localhost:1234/server-status?auto":      true,
		"https://customhost.com:8080/server-status?auto": true,
		"https://customhost.com:1234/server-status?auto": true,
		"https://127.0.0.1:8080/custom?auto":             true,
		"https://127.0.0.1:1234/custom?auto":             true,
		"https://localhost:8080/custom?auto":             true,
		"https://localhost:1234/custom?auto":             true,
		"https://customhost.com:8080/custom?auto":        true,
		"https://customhost.com:1234/custom?auto":        true,
	}

	for _, protocol := range protocols {
		for _, host := range hosts {
			for _, port := range ports {
				for _, path := range paths {
					endpoint := fmt.Sprintf("%s%s%s%s?auto", protocol, host, port, path)
					endpoints = append(endpoints, endpoint)
				}
			}
		}
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = endpoint
			require.NoError(t, cfg.Validate())
			_, ok := validEndpoints[cfg.Endpoint]
			require.True(t, ok)
		})
	}
}
