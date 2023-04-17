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

package syslogexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {

	tests := []struct {
		name string
		cfg  *Config
		err  string
	}{
		{
			name: "invalid Port",
			cfg: &Config{
				Port:     515444,
				Endpoint: "host.domain.com",
				Protocol: "rfc542",
				Network:  "udp",
			},
			err: "unsupported port: port is required, must be in the range 1-65535; " +
				"unsupported protocol: Only rfc5424 and rfc3164 supported",
		},
		{
			name: "invalid Endpoint",
			cfg: &Config{
				Port:     514,
				Endpoint: "",
				Protocol: "rfc5424",
				Network:  "udp",
			},
			err: "invalid endpoint: endpoint is required but it is not configured",
		},
		{
			name: "unsupported Network",
			cfg: &Config{
				Port:     514,
				Endpoint: "host.domain.com",
				Protocol: "rfc5424",
				Network:  "ftp",
			},
			err: "unsupported network: network is required, only tcp/udp supported",
		},
		{
			name: "Unsupported Protocol",
			cfg: &Config{
				Port:     514,
				Endpoint: "host.domain.com",
				Network:  "udp",
				Protocol: "rfc",
			},
			err: "unsupported protocol: Only rfc5424 and rfc3164 supported",
		},
	}
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			err := testInstance.cfg.Validate()
			if testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
