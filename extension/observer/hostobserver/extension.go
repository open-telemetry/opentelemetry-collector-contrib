// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver"

import (
	"context"
	"fmt"
	"runtime"
	"syscall"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

type hostObserver struct {
	*endpointswatcher.EndpointsWatcher
}

type endpointsLister struct {
	logger       *zap.Logger
	observerName string

	// For testing
	getConnections        func() ([]net.ConnectionStat, error)
	getProcess            func(pid int32) (*process.Process, error)
	collectProcessDetails func(proc *process.Process) (*processDetails, error)
}

var _ extension.Extension = (*hostObserver)(nil)

func newObserver(params extension.Settings, config *Config) (extension.Extension, error) {
	h := &hostObserver{
		EndpointsWatcher: endpointswatcher.New(
			endpointsLister{
				logger:                params.Logger,
				observerName:          params.ID.String(),
				getConnections:        getConnections,
				getProcess:            process.NewProcess,
				collectProcessDetails: collectProcessDetails,
			},
			config.RefreshInterval,
			params.Logger,
		),
	}

	return h, nil
}

func (h *hostObserver) Start(context.Context, component.Host) error {
	return nil
}

func (h *hostObserver) Shutdown(context.Context) error {
	h.StopListAndWatch()
	return nil
}

func (e endpointsLister) ListEndpoints() []observer.Endpoint {
	conns, err := e.getConnections()
	if err != nil {
		e.logger.Error("Could not get local network listeners", zap.Error(err))
		return nil
	}

	return e.collectEndpoints(conns)
}

func getConnections() (conns []net.ConnectionStat, err error) {
	// Skip UID lookup since it's not used by the observer, the method
	// is available only on linux. See https://github.com/shirou/gopsutil/pull/783
	// for details.
	if runtime.GOOS == "linux" {
		conns, err = net.ConnectionsWithoutUids("all")
	} else {
		conns, err = net.Connections("all")
	}

	return conns, err
}

func (e endpointsLister) collectEndpoints(conns []net.ConnectionStat) []observer.Endpoint {
	endpoints := make([]observer.Endpoint, 0, len(conns))
	connsByPID := make(map[int32][]*net.ConnectionStat)
	for i := range conns {
		c := conns[i]
		isIPSocket := c.Family == syscall.AF_INET || c.Family == syscall.AF_INET6
		isTCPOrUDP := c.Type == syscall.SOCK_STREAM || c.Type == syscall.SOCK_DGRAM
		// UDP doesn't have any status
		isUDPOrListening := c.Type == syscall.SOCK_DGRAM || c.Status == "LISTEN"
		// UDP is "listening" when it has a remote port of 0
		isTCPOrHasNoRemotePort := c.Type == syscall.SOCK_STREAM || c.Raddr.Port == 0

		if !isIPSocket || !isTCPOrUDP || !isUDPOrListening || !isTCPOrHasNoRemotePort {
			continue
		}

		// PID of 0 means that the listening file descriptor couldn't be mapped
		// back to a process's set of open file descriptors in /proc. Collect these
		// endpoints even though there's no process metadata available so users can
		// still do discovery rules on such sockets.
		if c.Pid == 0 {
			cd := collectConnectionDetails(&c)
			id := observer.EndpointID(
				fmt.Sprintf(
					"(%s)%s-%d-%s", e.observerName, cd.ip, cd.port, cd.transport,
				),
			)

			endpoints = append(endpoints, observer.Endpoint{
				ID:     id,
				Target: cd.target,
				Details: &observer.HostPort{
					Port:      cd.port,
					Transport: cd.transport,
					// TODO: Move this field to observer.Endpoint and
					// update receiver_creator to filter IPv4/IPv6.
					IsIPv6: cd.isIPv6,
				},
			})
			continue
		}

		connsByPID[c.Pid] = append(connsByPID[c.Pid], &c)
	}

	for pid, conns := range connsByPID {
		proc, err := e.getProcess(pid)
		if err != nil {
			e.logger.Warn("Could not examine process (it might have terminated already)")
			continue
		}

		pd, err := e.collectProcessDetails(proc)
		if err != nil {
			e.logger.Error("Failed collecting process details (skipping)",
				zap.Int32("pid", pid), zap.Error(err),
			)
			continue
		}

		for _, c := range conns {
			cd := collectConnectionDetails(c)

			id := observer.EndpointID(
				fmt.Sprintf(
					"(%s)%s-%d-%s-%d",
					e.observerName, cd.ip, cd.port, cd.transport, pid,
				),
			)

			e := observer.Endpoint{
				ID:     id,
				Target: cd.target,
				Details: &observer.HostPort{
					ProcessName: pd.name,
					Command:     pd.args,
					Port:        cd.port,
					Transport:   cd.transport,
					// TODO: Move this field to observer.Endpoint and
					// update receiver_creator to filter IPv4/IPv6.
					IsIPv6: cd.isIPv6,
				},
			}
			endpoints = append(endpoints, e)
		}
	}

	return endpoints
}

type connectionDetails struct {
	ip        string
	isIPv6    bool
	port      uint16
	target    string
	transport observer.Transport
}

func collectConnectionDetails(c *net.ConnectionStat) connectionDetails {
	ip := c.Laddr.IP
	// An IP addr of 0.0.0.0 (or "*" on darwin) means it listens on all
	// interfaces, including localhost, so use that since we can't
	// actually connect to 0.0.0.0.
	if ip == "0.0.0.0" || ip == "*" {
		ip = "127.0.0.1"
	}

	isIPv6 := false

	if c.Family == syscall.AF_INET6 {
		ip = "[" + ip + "]"
		isIPv6 = true
	}

	port := uint16(c.Laddr.Port)
	target := fmt.Sprintf("%s:%d", ip, port)
	protocol := portTypeToProtocol(c.Type)

	return connectionDetails{
		ip:        ip,
		isIPv6:    isIPv6,
		port:      port,
		target:    target,
		transport: protocol,
	}
}

type processDetails struct {
	name string
	args string
}

func collectProcessDetails(proc *process.Process) (*processDetails, error) {
	name, err := proc.Name()
	if err != nil {
		return nil, fmt.Errorf("could not get process name: %w", err)
	}

	args, err := proc.Cmdline()
	if err != nil {
		return nil, fmt.Errorf("could not get process args: %w", err)
	}

	return &processDetails{
		name: name,
		args: args,
	}, nil
}

func portTypeToProtocol(t uint32) observer.Transport {
	switch t {
	case syscall.SOCK_STREAM:
		return observer.ProtocolTCP
	case syscall.SOCK_DGRAM:
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}
