// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostobserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

// Tests observer with real connections on system.
func TestHostObserver(t *testing.T) {
	// TODO review if test should succeed on Windows
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	tests := []struct {
		name                    string
		protocol                observer.Transport
		setup                   func() ([]hostPort, mockNotifier)
		errorListingConnections bool
	}{
		{
			name:     "TCP ports",
			protocol: observer.ProtocolTCP,
			setup: func() ([]hostPort, mockNotifier) {
				tcpConns := openTestTCPPorts(t)
				mn := startAndStopObserver(t, nil)
				out := make([]hostPort, len(tcpConns))
				for i, conn := range tcpConns {
					out[i] = getHostAndPort(conn)
				}
				return out, mn
			},
		},
		{
			name:     "UDP ports",
			protocol: observer.ProtocolUDP,
			setup: func() ([]hostPort, mockNotifier) {
				udpConns := openTestUDPPorts(t)
				mn := startAndStopObserver(t, nil)
				out := make([]hostPort, len(udpConns))
				for i, conn := range udpConns {
					out[i] = getHostAndPort(conn)
				}
				return out, mn
			},
		},
		{
			name: "Fails to get connections",
			setup: func() ([]hostPort, mockNotifier) {
				mn := startAndStopObserver(t, func() (conns []psnet.ConnectionStat, err error) {
					return nil, errors.New("fails to list connections")
				})
				return nil, mn
			},
			errorListingConnections: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostPorts, notifier := tt.setup()
			if tt.errorListingConnections {
				require.Empty(t, notifier.endpointsMap)
				return
			}

			require.GreaterOrEqual(t, len(notifier.endpointsMap), len(hostPorts))

			for _, hp := range hostPorts {
				require.NoError(t, hp.err, "Failed to et host and port")

				host := hp.host
				port := hp.port
				isIPv6 := false
				if host[0] == ':' {
					isIPv6 = true
				}

				host = getExpectedHost(host, isIPv6)
				expectedID := observer.EndpointID(
					fmt.Sprintf(
						"(host_observer/1)%s-%s-%s-%d", host, port, tt.protocol, selfPid),
				)

				actualEndpoint := notifier.endpointsMap[expectedID]
				require.NotEmpty(t, actualEndpoint, "expected endpoint ID not found. ID: %v", expectedID)
				assert.Equal(t, expectedID, actualEndpoint.ID, "unexpected endpoint ID found")

				details, ok := actualEndpoint.Details.(*observer.HostPort)
				assert.True(t, ok, "failed to get Endpoint.Details")
				assert.Equal(t, filepath.Base(exe), details.ProcessName)
				assert.Equal(t, tt.protocol, details.Transport)
				assert.Equal(t, isIPv6, details.IsIPv6)
			}
		})
	}
}

type hostPort struct {
	host string
	port string
	err  error
}

func getHostAndPort(i any) hostPort {
	var host, port string
	var err error
	switch conn := i.(type) {
	case *net.TCPListener:
		host, port, err = net.SplitHostPort(conn.Addr().String())
	case *net.UDPConn:
		host, port, err = net.SplitHostPort(conn.LocalAddr().String())
	default:
		err = errors.New("failed to get host and port")
	}
	return hostPort{
		host: host,
		port: port,
		err:  err,
	}
}

func getExpectedHost(host string, isIPv6 bool) string {
	out := host

	if runtime.GOOS == "darwin" && host == "::" {
		out = "127.0.0.1"
	}

	if isIPv6 {
		out = "[" + out + "]"
	}
	return out
}

func startAndStopObserver(
	t *testing.T,
	getConnectionsOverride func() (conns []psnet.ConnectionStat, err error),
) mockNotifier {
	ml := endpointsLister{
		logger:                zap.NewNop(),
		observerName:          "host_observer/1",
		getConnections:        getConnections,
		getProcess:            process.NewProcess,
		collectProcessDetails: collectProcessDetails,
	}

	if getConnectionsOverride != nil {
		ml.getConnections = getConnectionsOverride
	}

	require.NotNil(t, ml.getConnections)
	require.NotNil(t, ml.getProcess)
	require.NotNil(t, ml.collectProcessDetails)

	h := &hostObserver{EndpointsWatcher: endpointswatcher.New(ml, 10*time.Second, zaptest.NewLogger(t))}

	mn := mockNotifier{map[observer.EndpointID]observer.Endpoint{}}

	ctx := context.Background()
	require.NoError(t, h.Start(ctx, componenttest.NewNopHost()))
	h.ListAndWatch(mn)

	time.Sleep(2 * time.Second) // Wait a bit to sync endpoints once.
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

func (m mockNotifier) ID() observer.NotifyID {
	return "mockNotifier"
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
		conn *psnet.ConnectionStat
		want connectionDetails
	}{
		{
			name: "TCPv4 connection",
			conn: &psnet.ConnectionStat{
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
			conn: &psnet.ConnectionStat{
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
			conn: &psnet.ConnectionStat{
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
			conn: &psnet.ConnectionStat{
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
			conn: &psnet.ConnectionStat{
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
		{
			name: "Listening on all interfaces (Darwin)",
			conn: &psnet.ConnectionStat{
				Family: syscall.AF_INET,
				Type:   syscall.SOCK_DGRAM,
				Laddr: psnet.Addr{
					IP:   "*",
					Port: 8080,
				},
			},
			want: connectionDetails{
				ip:        "127.0.0.1",
				isIPv6:    false,
				port:      uint16(8080),
				target:    "127.0.0.1:8080",
				transport: observer.ProtocolUDP,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, collectConnectionDetails(tt.conn))
		})
	}
}

func TestCollectEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		conns       []psnet.ConnectionStat
		newProc     func(pid int32) (*process.Process, error)
		procDetails func(proc *process.Process) (*processDetails, error)
		want        []observer.Endpoint
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
					Details: &observer.HostPort{
						ProcessName: "",
						Port:        80,
						Transport:   observer.ProtocolTCP,
						IsIPv6:      false,
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
			name: "Fails to get process that no longer exists",
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
			newProc: func(int32) (*process.Process, error) {
				return nil, errors.New("always fail")
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
			newProc: func(pid int32) (*process.Process, error) {
				return &process.Process{Pid: pid}, nil
			},
			procDetails: func(_ *process.Process) (*processDetails, error) {
				return nil, errors.New("always fail")
			},
			want: []observer.Endpoint{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := endpointsLister{
				logger:                zap.NewNop(),
				getProcess:            process.NewProcess,
				collectProcessDetails: collectProcessDetails,
			}

			if tt.procDetails != nil {
				e.collectProcessDetails = tt.procDetails
			}

			if tt.newProc != nil {
				e.getProcess = tt.newProc
			}

			require.NotNil(t, e.collectProcessDetails)
			require.NotNil(t, e.getProcess)
			assert.Equal(t, tt.want, e.collectEndpoints(tt.conns))
		})
	}
}
