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

// +build !darwin

package hostobserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	psnet "github.com/shirou/gopsutil/net"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestHostObserver(t *testing.T) {
	tcpConns := openTestTCPPorts(t)
	udpConns := openTestUDPPorts(t)

	mn := startAndStopObserver(t)

	assert.True(t, len(mn.endpointsMap) >= len(tcpConns)+len(udpConns))

	t.Run("TCP Ports", func(t *testing.T) {
		for _, conn := range tcpConns {
			host, port, err := net.SplitHostPort(conn.Addr().String())
			assert.NoError(t, err, "Failed to et host and port")

			expectedID := observer.EndpointID(
				fmt.Sprintf(
					"(host_observer/1)%s-%s-%s-%d", host, port, observer.ProtocolTCP, selfPid),
			)
			actualEndpoint := mn.endpointsMap[expectedID]
			assert.NotEmpty(t, actualEndpoint, "expected endpoint ID not found")
			assert.Equal(t, expectedID, actualEndpoint.ID, "unexpected endpoint ID found")

			details, ok := actualEndpoint.Details.(observer.HostPort)
			assert.True(t, ok, "failed to get Endpoint.Details")
			assert.Equal(t, filepath.Base(exe), details.Name)
			assert.Equal(t, observer.ProtocolTCP, details.Transport)
			if host[0] == ':' {
				assert.Equal(t, true, details.IsIPv6)
			} else {
				assert.Equal(t, false, details.IsIPv6)
			}
		}
	})

	t.Run("UDP Ports", func(t *testing.T) {
		for _, conn := range udpConns {
			host, port, err := net.SplitHostPort(conn.LocalAddr().String())
			assert.NoError(t, err, "Failed to et host and port")

			expectedID := observer.EndpointID(
				fmt.Sprintf(
					"(host_observer/1)%s-%s-%s-%d", host, port, observer.ProtocolUDP, selfPid),
			)

			actualEndpoint := mn.endpointsMap[expectedID]
			assert.NotEmpty(t, actualEndpoint, "expected endpoint ID not found")
			assert.Equal(t, expectedID, actualEndpoint.ID, "unexpected endpoint ID found")

			details, ok := actualEndpoint.Details.(observer.HostPort)
			assert.True(t, ok, "failed to get Endpoint.Details")
			assert.Equal(t, filepath.Base(exe), details.Name)
			assert.Equal(t, observer.ProtocolUDP, details.Transport)
			if host[0] == ':' {
				assert.Equal(t, true, details.IsIPv6)
			} else {
				assert.Equal(t, false, details.IsIPv6)
			}
		}
	})
}

func startAndStopObserver(t *testing.T) mockNotifier {
	ml := endpointsLister{
		logger:       zap.NewNop(),
		observerName: "host_observer/1",
	}

	h := &hostObserver{
		EndpointsWatcher: observer.EndpointsWatcher{
			RefreshInterval: 1 * time.Second,
			Endpointslister: ml,
		},
	}

	mn := mockNotifier{map[observer.EndpointID]observer.Endpoint{}}

	ctx := context.Background()
	require.NoError(t, h.Start(ctx, componenttest.NewNopHost()))
	h.ListAndWatch(mn)

	time.Sleep(1500 * time.Millisecond)
	require.NoError(t, h.Shutdown(ctx))

	return mn
}

var (
	exe, _  = os.Executable()
	selfPid = os.Getpid()
)

func openTestTCPPorts(t *testing.T) []*net.TCPListener {
	tcpLocalhost, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	assert.NoError(t, err)

	tcpV6Localhost, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("[::]"),
		Port: 0,
	})
	assert.NoError(t, err)

	tcpAllPorts, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 0,
	})
	assert.NoError(t, err)

	return []*net.TCPListener{
		tcpLocalhost,
		tcpV6Localhost,
		tcpAllPorts,
	}
}

func openTestUDPPorts(t *testing.T) []*net.UDPConn {
	udpLocalhost, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	assert.NoError(t, err)

	udpV6Localhost, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("[::]"),
		Port: 0,
	})
	assert.NoError(t, err)

	udpAllPorts, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 0,
	})
	assert.NoError(t, err)

	return []*net.UDPConn{
		udpLocalhost,
		udpV6Localhost,
		udpAllPorts,
	}
}

type mockNotifier struct {
	endpointsMap map[observer.EndpointID]observer.Endpoint
}

func (m mockNotifier) OnAdd(added []observer.Endpoint) {
	for _, e := range added {
		m.endpointsMap[e.ID] = e
	}
}

func (m mockNotifier) OnRemove(removed []observer.Endpoint) {
	for _, e := range removed {
		delete(m.endpointsMap, e.ID)
	}
}

func (m mockNotifier) OnChange(changed []observer.Endpoint) {
	for _, e := range changed {
		m.endpointsMap[e.ID] = e
	}
}

func TestPortTypeToProtocol(t *testing.T) {
	tests := []struct {
		name     string
		portType uint32
		want     observer.Transport
	}{
		{
			name:     "TCP",
			portType: syscall.SOCK_STREAM,
			want:     observer.ProtocolTCP,
		},
		{
			name:     "UDP",
			portType: syscall.SOCK_DGRAM,
			want:     observer.ProtocolUDP,
		},
		{
			name:     "Unsupported",
			portType: 999,
			want:     observer.ProtocolUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portTypeToProtocol(tt.portType); got != tt.want {
				t.Errorf("portTypeToProtocol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectConnectionDetails(t *testing.T) {
	tests := []struct {
		name string
		conn psnet.ConnectionStat
		want connectionDetails
	}{
		{
			name: "TCPv4 connection",
			conn: psnet.ConnectionStat{
				Family: syscall.AF_INET,
				Type:   syscall.SOCK_STREAM,
				Laddr: psnet.Addr{
					IP:   "123.123.99.0",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "123.123.99.0",
				isIPv6:    false,
				port:      uint16(8080),
				target:    "123.123.99.0:8080",
				transport: observer.ProtocolTCP,
			},
		},
		{
			name: "TCPv6 connection",
			conn: psnet.ConnectionStat{
				Family: syscall.AF_INET6,
				Type:   syscall.SOCK_STREAM,
				Laddr: psnet.Addr{
					IP:   "123.123.99.0",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "[123.123.99.0]",
				isIPv6:    true,
				port:      uint16(8080),
				target:    "[123.123.99.0]:8080",
				transport: observer.ProtocolTCP,
			},
		},
		{
			name: "TCPv4 connection - 0.0.0.0",
			conn: psnet.ConnectionStat{
				Family: syscall.AF_INET,
				Type:   syscall.SOCK_STREAM,
				Laddr: psnet.Addr{
					IP:   "0.0.0.0",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "127.0.0.1",
				isIPv6:    false,
				port:      uint16(8080),
				target:    "127.0.0.1:8080",
				transport: observer.ProtocolTCP,
			},
		},
		{
			name: "UDPv4 connection",
			conn: psnet.ConnectionStat{
				Family: syscall.AF_INET,
				Type:   syscall.SOCK_DGRAM,
				Laddr: psnet.Addr{
					IP:   "123.123.99.0",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "123.123.99.0",
				isIPv6:    false,
				port:      uint16(8080),
				target:    "123.123.99.0:8080",
				transport: observer.ProtocolUDP,
			},
		},
		{
			name: "UDPv6 connection",
			conn: psnet.ConnectionStat{
				Family: syscall.AF_INET6,
				Type:   syscall.SOCK_DGRAM,
				Laddr: psnet.Addr{
					IP:   "123.123.99.0",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "[123.123.99.0]",
				isIPv6:    true,
				port:      uint16(8080),
				target:    "[123.123.99.0]:8080",
				transport: observer.ProtocolUDP,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collectConnectionDetails(&tt.conn); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectConnectionDetails() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectEndpoints(t *testing.T) {
	tests := []struct {
		name                       string
		conns                      []psnet.ConnectionStat
		overrideProcessInfoMethods func()
		want                       []observer.Endpoint
	}{
		{
			name: "Listening TCP socket without process info",
			conns: []psnet.ConnectionStat{
				{
					Family: syscall.AF_INET,
					Type:   syscall.SOCK_STREAM,
					Laddr: psnet.Addr{
						IP:   "123.345.567.789",
						Port: 80,
					},
					Status: "LISTEN",
					Pid:    0,
				},
			},
			want: []observer.Endpoint{
				{
					ID:     observer.EndpointID("()123.345.567.789-80-TCP"),
					Target: "123.345.567.789:80",
					Details: observer.HostPort{
						Name:      "",
						Port:      80,
						Transport: observer.ProtocolTCP,
						IsIPv6:    false,
					},
				},
			},
		},
		{
			name: "TCP socket that's not listening",
			conns: []psnet.ConnectionStat{
				{
					Family: syscall.AF_INET,
					Type:   syscall.SOCK_STREAM,
					Laddr: psnet.Addr{
						IP:   "123.345.567.789",
						Port: 80,
					},
					Status: "ESTABLISHED",
					Pid:    0,
				},
			},
			want: []observer.Endpoint{},
		},
		{
			name: "Fails to get process info",
			conns: []psnet.ConnectionStat{
				{
					Family: syscall.AF_INET,
					Type:   syscall.SOCK_STREAM,
					Laddr: psnet.Addr{
						IP:   "123.345.567.789",
						Port: 80,
					},
					Status: "LISTEN",
					Pid:    9999,
				},
			},
			want: []observer.Endpoint{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := endpointsLister{
				logger: zap.NewNop(),
			}
			if got := e.collectEndpoints(tt.conns); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectEndpoints() = %v, want %v", got, tt.want)
			}
		})
	}
}
