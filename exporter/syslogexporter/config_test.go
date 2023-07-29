// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
