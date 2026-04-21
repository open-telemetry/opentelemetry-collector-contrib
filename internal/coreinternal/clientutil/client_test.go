// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
)

type fakeAddr string

func (s fakeAddr) String() string {
	return string(s)
}

func (fakeAddr) Network() string {
	return "tcp"
}

func TestAddress(t *testing.T) {
	tests := []struct {
		name   string
		client client.Info
		want   string
	}{
		{
			name: "UDPAddr",
			client: client.Info{
				Addr: &net.UDPAddr{
					IP:   net.IPv4(192, 0, 2, 1),
					Port: 1234,
				},
			},
			want: "192.0.2.1",
		},
		{
			name: "TCPAddr",
			client: client.Info{
				Addr: &net.TCPAddr{
					IP:   net.IPv4(192, 0, 2, 2),
					Port: 1234,
				},
			},
			want: "192.0.2.2",
		},
		{
			name: "IPAddr",
			client: client.Info{
				Addr: &net.IPAddr{
					IP: net.IPv4(192, 0, 2, 3),
				},
			},
			want: "192.0.2.3",
		},
		{
			name: "fake_addr_with_port",
			client: client.Info{
				Addr: fakeAddr("1.1.1.1:3200"),
			},
			want: "1.1.1.1",
		},
		{
			name: "fake_addr_without_port",
			client: client.Info{
				Addr: fakeAddr("1.1.1.1"),
			},
			want: "1.1.1.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Address(tt.client))
		})
	}
}
