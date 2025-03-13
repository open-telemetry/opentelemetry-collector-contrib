// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package efa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPv6LinkLocalToMAC(t *testing.T) {
	tests := []struct {
		name    string
		ipv6    string
		wantMAC string
		wantErr bool
	}{
		{
			name:    "Valid link-local address",
			ipv6:    "fe80::200:ff:fe00:1",
			wantMAC: "00:00:00:00:00:01",
			wantErr: false,
		},
		{
			name:    "Invalid IPv6 address",
			ipv6:    "not-an-ip",
			wantMAC: "",
			wantErr: true,
		},
		{
			name:    "Non link-local address",
			ipv6:    "2001:db8::1",
			wantMAC: "",
			wantErr: true,
		},
		{
			name:    "Non EUI-64 format",
			ipv6:    "fe80::1",
			wantMAC: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMAC, err := IPv6LinkLocalToMAC(tt.ipv6)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tt.wantMAC, gotMAC)
			}
		})
	}
}
