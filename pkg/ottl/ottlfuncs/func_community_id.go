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

	return communityID(
		args.SourceIP,
		args.SourcePort,
		args.DestinationIP,
		args.DestinationPort,
		args.Protocol,
		args.Seed,
	)
}

func communityID[K any](
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

		protocolValue := uint8(6) // defaults to TCP
		if !protocol.IsEmpty() {
			protocolStr, err := protocol.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to get protocol: %w", err)
			}

			resolvedProtocolValue, err := resolveProtocolValue(protocolStr)
			if err != nil {
				return nil, err
			}
			protocolValue = resolvedProtocolValue
		}

		srcIPAddr := net.ParseIP(srcIPValue)
		if srcIPAddr == nil {
			return nil, fmt.Errorf("invalid source IP: %s", srcIPValue)
		}

		dstIPAddr := net.ParseIP(dstIPValue)
		if dstIPAddr == nil {
			return nil, fmt.Errorf("invalid destination IP: %s", dstIPValue)
		}

		// Get seed value (default: 0) if applied
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
		shouldSwap := false
		if len(srcIPBytes) != len(dstIPBytes) {
			shouldSwap = len(srcIPBytes) > len(dstIPBytes)
		} else if cmp := bytesCompare(srcIPBytes, dstIPBytes); cmp > 0 {
			shouldSwap = true
		} else if cmp == 0 && srcPort > dstPort {
			shouldSwap = true
		}
		if shouldSwap {
			srcIPBytes, dstIPBytes = dstIPBytes, srcIPBytes
			srcPortForHash, dstPortForHash = dstPortForHash, srcPortForHash
		}

		// Build the flow tuple
		// Format: <seed:2><protocol:1><src_ip><dst_ip><src_port:2><dst_port:2>
		return flow(srcIPBytes, srcPortForHash, dstIPBytes, dstPortForHash, protocolValue, seedValue), nil
	}, nil
}

func resolveProtocolValue(protoStr string) (uint8, error) {
	protoMap := map[string]uint8{"ICMP": 1, "TCP": 6, "UDP": 17, "RSVP": 46, "ICMP6": 58, "SCTP": 132}
	protocolValue := protoMap[protoStr]

	if protocolValue == 0 {
		return 0, fmt.Errorf("unsupported protocol: %s", protoStr)
	}
	return protocolValue, nil
}

func bytesCompare(a, b []byte) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func flow(srcIPBytes net.IP, srcPortForHash uint16, dstIPBytes net.IP, dstPortForHash uint16, protolValue uint8, seedValue uint16) string {
	// Add seed (2 bytes, network order)
	flowTuple := make([]byte, 2)
	binary.BigEndian.PutUint16(flowTuple, seedValue)

	// Add source, destination IPs and 1-byte protocol
	flowTuple = append(flowTuple, srcIPBytes...)
	flowTuple = append(flowTuple, dstIPBytes...)
	flowTuple = append(flowTuple, protolValue, 0)

	// Add source and destination ports (2 bytes each, network order)
	srcPortBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(srcPortBytes, srcPortForHash)
	flowTuple = append(flowTuple, srcPortBytes...)

	dstPortBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(dstPortBytes, dstPortForHash)
	flowTuple = append(flowTuple, dstPortBytes...)

	// Generate the SHA1 hash
	//gosec:disable G401 -- we are not using SHA1 for security, but for generating unique identifier, conflicts will be solved with the seed
	hashBytes := sha1.Sum(flowTuple)

	// Add version prefix (1) and return
	return "1:" + base64.StdEncoding.EncodeToString(hashBytes[:])
}
