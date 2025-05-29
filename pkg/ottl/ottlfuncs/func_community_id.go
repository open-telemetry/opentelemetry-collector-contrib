// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	//gosec:disable G505 -- SHA1 is intentionally used for generating unique identifiers
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type CommunityIDArguments[K any] struct {
	SourceIP        ottl.StringGetter[K]
	SourcePort      ottl.IntGetter[K]
	DestinationIP   ottl.StringGetter[K]
	DestinationPort ottl.IntGetter[K]
	Protocol        ottl.Optional[ottl.StringGetter[K]]
	Seed            ottl.Optional[ottl.IntGetter[K]]
}

func NewCommunityIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("CommunityID", &CommunityIDArguments[K]{}, createCommunityIDFunction[K])
}

func createCommunityIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*CommunityIDArguments[K])

	if !ok {
		return nil, errors.New("CommunityIDFactory args must be of type *CommunityIDArguments[K]")
	}

	return CommunityIDHash(
		args.SourceIP,
		args.SourcePort,
		args.DestinationIP,
		args.DestinationPort,
		args.Protocol,
		args.Seed,
	)
}

func CommunityIDHash[K any](
	sourceIP ottl.StringGetter[K],
	sourcePort ottl.IntGetter[K],
	destinationIP ottl.StringGetter[K],
	destinationPort ottl.IntGetter[K],
	protocol ottl.Optional[ottl.StringGetter[K]],
	seed ottl.Optional[ottl.IntGetter[K]],
) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		srcIPValue, err := sourceIP.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get source IP: %w", err)
		}

		dstIPValue, err := destinationIP.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get destination IP: %w", err)
		}

		srcPort, err := sourcePort.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get source port: %w", err)
		}

		dstPort, err := destinationPort.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get destination port: %w", err)
		}

		protolValue := 6 // default to TCP
		if !protocol.IsEmpty() {
			protoStr, err := protocol.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to get protocol: %w", err)
			}

			resolvedProto, err := resolveProtocolValue(protoStr)
			if err != nil {
				return nil, err
			}
			protolValue = resolvedProto
		}

		// Parse IPs
		srcIPAddr := net.ParseIP(srcIPValue)
		if srcIPAddr == nil {
			return nil, fmt.Errorf("invalid source IP: %s", srcIPValue)
		}

		dstIPAddr := net.ParseIP(dstIPValue)
		if dstIPAddr == nil {
			return nil, fmt.Errorf("invalid destination IP: %s", dstIPValue)
		}

		// Get seed value (default: 0)
		seedValue := uint16(0)
		if !seed.IsEmpty() {
			seedInt, err := seed.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to get seed: %w", err)
			}
			seedValue = uint16(seedInt)
		}

		// Determine a flow direction and normalize
		srcIPBytes := srcIPAddr.To4()
		dstIPBytes := dstIPAddr.To4()
		srcPortForHash, dstPortForHash := uint16(srcPort), uint16(dstPort)

		// IPv4 (4-bytes) or IPv6 (16-bytes) mapped address
		if srcIPBytes == nil {
			srcIPBytes = srcIPAddr.To16()
		}

		if dstIPBytes == nil {
			dstIPBytes = dstIPAddr.To16()
		}

		// Determine a flow direction for normalization
		// If source IP is greater than destination IP, or if IPs are equal and source port is greater than destination port,
		// then swap source and destination to normalize the flow direction
		if result := compareIPs(srcIPValue, dstIPValue); result > 0 {
			srcIPBytes, dstIPBytes = dstIPBytes, srcIPBytes
			srcPortForHash, dstPortForHash = dstPortForHash, srcPortForHash
		} else if result == 0 && srcPort > dstPort {
			srcPortForHash, dstPortForHash = uint16(srcPort), uint16(dstPort)
		}

		// Build the flow tuple
		// Format: <seed:2><protocol:1><src_ip><dst_ip><src_port:2><dst_port:2>
		return generateFlowTuple(srcIPBytes, dstIPBytes, srcPortForHash, dstPortForHash, protolValue, seedValue), nil
	}, nil
}

// Parse the protocol
func resolveProtocolValue(protoStr string) (int, error) {
	proto := -1
	protoStr = strings.ToUpper(protoStr)
	switch protoStr {
	case "TCP":
		proto = 6
	case "UDP":
		proto = 17
	case "ICMP":
		proto = 1
	case "ICMPV6":
		proto = 58
	case "SCTP":
		proto = 132
	case "RSVP":
		proto = 46
	default:
		return proto, fmt.Errorf("unsupported protocol: %s", protoStr)
	}
	return proto, nil
}

// compareIPs compares two IP addresses
// Returns:
//
//	-1 if ip1 < ip2
//	 0 if equal
//	 1 is if ip1 > ip2
func compareIPs(ipSource1 string, ipSource2 string) int {
	// skip error check since already validated
	ip1, _ := netip.ParseAddr(ipSource1)
	ip2, _ := netip.ParseAddr(ipSource2)

	return ip1.Compare(ip2)
}

func generateFlowTuple(srcIPBytes net.IP, dstIPBytes net.IP, srcPortForHash uint16, dstPortForHash uint16, protolValue int, seedValue uint16) string {
	flowTuple := make([]byte, 0, 2+1+len(srcIPBytes)+len(dstIPBytes)+2+2)

	// Add seed (2 bytes, network order)
	seedBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(seedBytes, seedValue)
	flowTuple = append(flowTuple, seedBytes...)

	// Add protocol (1 byte)
	flowTuple = append(flowTuple, byte(protolValue))

	// Add source and destination IPs
	flowTuple = append(flowTuple, srcIPBytes...)
	flowTuple = append(flowTuple, dstIPBytes...)

	// Add source and destination ports (2 bytes each, network order)
	srcPortBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(srcPortBytes, srcPortForHash)
	flowTuple = append(flowTuple, srcPortBytes...)

	dstPortBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(dstPortBytes, dstPortForHash)
	flowTuple = append(flowTuple, dstPortBytes...)

	// Generate the SHA1 hash
	//gosec:disable G401 -- we are not using SHA1 for security, but for generating unique identifier, conflicts will be solved with the seed
	hash := sha1.New()
	hash.Write(flowTuple)
	hashBytes := hash.Sum(nil)

	// Encode with Base64
	encoded := base64.StdEncoding.EncodeToString(hashBytes)

	// Add version prefix (1) and return
	return "1:" + encoded
}
