// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"bytes"
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

var protocolMap = map[string]uint8{
	"ICMP":  1,
	"TCP":   6,
	"UDP":   17,
	"RSVP":  46,
	"ICMP6": 58,
	"SCTP":  132,
}

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
	), nil
}

type communityIDParams struct {
	srcIPBytes []byte
	dstIPBytes []byte
	srcPort    uint16
	dstPort    uint16
	protocol   uint8
	seed       uint16
}

func communityID[K any](
	sourceIP ottl.StringGetter[K],
	sourcePort ottl.IntGetter[K],
	destinationIP ottl.StringGetter[K],
	destinationPort ottl.IntGetter[K],
	protocol ottl.Optional[ottl.StringGetter[K]],
	seed ottl.Optional[ottl.IntGetter[K]],
) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		params, err := validateAndExtractParams(ctx, tCtx, sourceIP, sourcePort, destinationIP, destinationPort, protocol, seed)
		if err != nil {
			return nil, err
		}

		// Determine a flow direction for normalization
		// If source IP is greater than destination IP, or if IPs are equal and source port is greater than destination port,
		// then swap source and destination to normalize the flow direction
		srcIPBytes, dstIPBytes := params.srcIPBytes, params.dstIPBytes
		srcPort, dstPort := params.srcPort, params.dstPort

		shouldSwap := false
		if len(srcIPBytes) != len(dstIPBytes) {
			shouldSwap = len(srcIPBytes) > len(dstIPBytes)
		} else if cmp := bytes.Compare(srcIPBytes, dstIPBytes); cmp > 0 {
			shouldSwap = true
		} else if cmp == 0 && srcPort > dstPort {
			shouldSwap = true
		}
		if shouldSwap {
			srcIPBytes, dstIPBytes = dstIPBytes, srcIPBytes
			srcPort, dstPort = dstPort, srcPort
		}

		// Build the flow, format: <seed:2><protocol:1><src_ip><dst_ip><src_port:2><dst_port:2>
		return flow(srcIPBytes, srcPort, dstIPBytes, dstPort, params.protocol, params.seed), nil
	}
}

func validateAndExtractParams[K any](
	ctx context.Context,
	tCtx K,
	sourceIP ottl.StringGetter[K],
	sourcePort ottl.IntGetter[K],
	destinationIP ottl.StringGetter[K],
	destinationPort ottl.IntGetter[K],
	protocol ottl.Optional[ottl.StringGetter[K]],
	seed ottl.Optional[ottl.IntGetter[K]],
) (*communityIDParams, error) {
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
	if srcPort < 1 || srcPort > 65535 {
		return nil, fmt.Errorf("source port must be between 1 and 65535, got %d", srcPort)
	}

	dstPort, err := destinationPort.Get(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get destination port: %w", err)
	}
	if dstPort < 1 || dstPort > 65535 {
		return nil, fmt.Errorf("destination port must be between 1 and 65535, got %d", dstPort)
	}

	protocolValue := protocolMap["TCP"] // defaults to TCP
	if !protocol.IsEmpty() {
		protocolStr, err := protocol.Get().Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get protocol: %w", err)
		}

		protocolValue = protocolMap[protocolStr]
		if protocolValue == 0 {
			return nil, fmt.Errorf("unsupported protocol: %s", protocolStr)
		}
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
		if seedInt < 0 || seedInt > 65535 {
			return nil, fmt.Errorf("seed must be between 0 and 65535, got %d", seedInt)
		}
		seedValue = uint16(seedInt)
	}

	srcIPBytes := srcIPAddr.To4()
	dstIPBytes := dstIPAddr.To4()

	if srcIPBytes == nil {
		srcIPBytes = srcIPAddr.To16()
	}

	if dstIPBytes == nil {
		dstIPBytes = dstIPAddr.To16()
	}

	return &communityIDParams{
		srcIPBytes: srcIPBytes,
		dstIPBytes: dstIPBytes,
		srcPort:    uint16(srcPort),
		dstPort:    uint16(dstPort),
		protocol:   protocolValue,
		seed:       seedValue,
	}, nil
}

func flow(srcIPBytes net.IP, srcPortForHash uint16, dstIPBytes net.IP, dstPortForHash uint16, protoValue uint8, seedValue uint16) string {
	// Add seed (2 bytes, network order)
	flowTuple := make([]byte, 2)
	binary.BigEndian.PutUint16(flowTuple, seedValue)

	// Add source, destination IPs and 1-byte protocol
	flowTuple = append(flowTuple, srcIPBytes...)
	flowTuple = append(flowTuple, dstIPBytes...)
	flowTuple = append(flowTuple, protoValue, 0)

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
