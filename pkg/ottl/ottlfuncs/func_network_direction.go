// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"net"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	// direction
	directionInternal = "internal"
	directionExternal = "external"
	directionInbound  = "inbound"
	directionOutbound = "outbound"

	// netwroks
	loopbackNamedNetwork           = "loopback"
	globalUnicastNamedNetwork      = "global_unicast"
	unicastNamedNetwork            = "unicast"
	linkLocalUnicastNamedNetwork   = "link_local_unicast"
	interfaceLocalNamedNetwork     = "interface_local_multicast"
	linkLocalMulticastNamedNetwork = "link_local_multicast"
	multicastNamedNetwork          = "multicast"
	unspecifiedNamedNetwork        = "unspecified"
	privateNamedNetwork            = "private"
	publicNamedNetwork             = "public"
)

type NetworkDirectionArguments[K any] struct {
	SourceIP         ottl.StringGetter[K]
	DestinationIP    ottl.StringGetter[K]
	InternalNetworks ottl.Optional[[]string]
}

func NewNetworkDirectionFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("NetworkDirection", &NetworkDirectionArguments[K]{}, createNetworkDirectionFunction[K])
}

func createNetworkDirectionFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*NetworkDirectionArguments[K])
	if !ok {
		return nil, fmt.Errorf("URLFactory args must be of type *NetworkDirectionArguments[K]")
	}

	return networkDirection(args.SourceIP, args.DestinationIP, args.InternalNetworks), nil //revive:disable-line:var-naming
}

func networkDirection[K any](sourceIP ottl.StringGetter[K], destinationIP ottl.StringGetter[K], internalNetworksOptional ottl.Optional[[]string]) ottl.ExprFunc[K] { //revive:disable-line:var-naming
	var internalNetworks []string
	if !internalNetworksOptional.IsEmpty() {
		for _, network := range internalNetworksOptional.Get() {
			if len(network) > 0 {
				internalNetworks = append(internalNetworks, network)
			}
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		sourceIPString, err := sourceIP.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed parsing source IP: %w", err)
		}

		if sourceIPString == "" {
			return nil, fmt.Errorf("source IP cannot be empty")
		}

		destinationIPString, err := destinationIP.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed parsing destination IP: %w", err)
		}

		if destinationIPString == "" {
			return nil, fmt.Errorf("destination IP cannot be empty")
		}

		sourceAddress := net.ParseIP(sourceIPString)
		if sourceAddress == nil {
			return nil, fmt.Errorf("source IP %q is not a valid address", sourceIPString)
		}

		sourceInternal, err := isInternalIP(sourceAddress, internalNetworks)
		if err != nil {
			return nil, fmt.Errorf("failed determining whether source IP is internal: %w", err)
		}

		destinationAddress := net.ParseIP(destinationIPString)
		if destinationAddress == nil {
			return nil, fmt.Errorf("destination IP %q is not a valid address", destinationIPString)
		}

		destinationInternal, err := isInternalIP(destinationAddress, internalNetworks)
		if err != nil {
			return nil, fmt.Errorf("failed determining whether destination IP is internal: %w", err)
		}

		if sourceInternal && destinationInternal {
			return directionInternal, nil
		}
		if sourceInternal {
			return directionOutbound, nil
		}
		if destinationInternal {
			return directionInbound, nil
		}
		return directionExternal, nil
	}
}

func isInternalIP(addr net.IP, networks []string) (bool, error) {
	for _, network := range networks {
		inNetwork, err := isIPInNetwork(addr, network)
		if err != nil {
			return false, err
		}

		if inNetwork {
			return true, nil
		}
	}
	return false, nil
}

func isIPInNetwork(addr net.IP, network string) (bool, error) {
	switch network {
	case loopbackNamedNetwork:
		return addr.IsLoopback(), nil
	case globalUnicastNamedNetwork:
		return addr.IsGlobalUnicast(), nil
	case linkLocalUnicastNamedNetwork:
		return addr.IsLinkLocalUnicast(), nil
	case linkLocalMulticastNamedNetwork:
		return addr.IsLinkLocalMulticast(), nil
	case interfaceLocalNamedNetwork:
		return addr.IsInterfaceLocalMulticast(), nil
	case multicastNamedNetwork:
		return addr.IsMulticast(), nil
	case privateNamedNetwork:
		return isPrivateNetwork(addr), nil
	case publicNamedNetwork:
		return isPublicNetwork(addr), nil
	case unspecifiedNamedNetwork:
		return addr.IsUnspecified(), nil

	}

	// cidr range
	return isInRange(addr, network)
}

func isPrivateNetwork(addr net.IP) bool {
	isAddrInRange, _ := isInRange(addr, "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fd00::/8")
	return isAddrInRange
}

func isPublicNetwork(addr net.IP) bool {
	if isPrivateNetwork(addr) ||
		addr.IsLoopback() ||
		addr.IsUnspecified() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.Equal(net.IPv4bcast) {
		return false
	}

	return false
}

func isInRange(addr net.IP, networks ...string) (bool, error) {
	for _, network := range networks {
		_, mask, err := net.ParseCIDR(network)
		if err != nil {
			return false, fmt.Errorf("invalid network definition for %q", network)
		}

		if mask.Contains(addr) {
			return true, nil
		}
	}

	return false, nil
}
