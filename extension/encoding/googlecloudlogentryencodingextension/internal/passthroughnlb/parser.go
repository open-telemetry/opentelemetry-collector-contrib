// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package passthroughnlb contains utilities for parsing Google Cloud Passthrough External and Internal Network Load Balancer logs.
package passthroughnlb // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/passthroughnlb"

import (
	"errors"
	"fmt"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	// ConnectionsLogNameSuffix identifies load balancer connection logs in the logName field.
	ConnectionsLogNameSuffix = "loadbalancing.googleapis.com%2Fflows"

	externalLoadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.ExternalNetworkLoadBalancerLogEntry"
	internalLoadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.InternalNetworkLoadBalancerLogEntry"

	gcpPassthroughNLBPacketsStartTime = "gcp.load_balancing.passthrough_nlb.packets.start_time" // #nosec G101
	gcpPassthroughNLBPacketsEndTime   = "gcp.load_balancing.passthrough_nlb.packets.end_time"   // #nosec G101
	gcpPassthroughNLBBytesReceived    = "gcp.load_balancing.passthrough_nlb.bytes_received"     // #nosec G101
	gcpPassthroughNLBBytesSent        = "gcp.load_balancing.passthrough_nlb.bytes_sent"         // #nosec G101
	gcpPassthroughNLBPacketsReceived  = "gcp.load_balancing.passthrough_nlb.packets_received"   // #nosec G101
	gcpPassthroughNLBPacketsSent      = "gcp.load_balancing.passthrough_nlb.packets_sent"       // #nosec G101
	gcpPassthroughNLBRTT              = "gcp.load_balancing.passthrough_nlb.rtt"                // #nosec G101
)

var (
	errUnmarshalPayload  = errors.New("failed to unmarshal Passthrough NLB log payload")
	errUnexpectedLogType = errors.New("unexpected log type")
	errBytesReceived     = errors.New("failed to add bytes received")
	errBytesSent         = errors.New("failed to add bytes sent")
	errPacketsReceived   = errors.New("failed to packets received")
	errPacketsSent       = errors.New("failed to add packets sent")
	errRTT               = errors.New("failed to add RTT")
	errServerAddress     = errors.New("failed to set server address")
)

type loadBalancerLog struct {
	Type       string      `json:"@type"`
	Connection *connection `json:"connection"`
	StartTime  *time.Time  `json:"startTime"`
	EndTime    *time.Time  `json:"endTime"`

	// BytesReceived, BytesSent, PacketsReceived, PacketsSent are string-encoded 64-bit integers.
	// Although the official documentation (https://docs.cloud.google.com/load-balancing/docs/network/networklb-monitoring)
	// specifies these fields as integers, the actual
	// JSON payload from Cloud Logging seems to stringify int64/uint64 values to
	// preserve precision
	BytesReceived   string `json:"bytesReceived"`
	BytesSent       string `json:"bytesSent"`
	PacketsReceived string `json:"packetsReceived"`
	PacketsSent     string `json:"packetsSent"`
	RTT             string `json:"rtt"`
}

type connection struct {
	ClientIP   string `json:"clientIp"`
	ClientPort *int64 `json:"clientPort"`
	Protocol   *int64 `json:"protocol"`
	ServerIP   string `json:"serverIp"`
	ServerPort *int64 `json:"serverPort"`
}

// ParsePayloadIntoAttributes unmarshals the provided payload into the supplied attribute map.
func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log loadBalancerLog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("%w: %w", errUnmarshalPayload, err)
	}

	if log.Type != externalLoadBalancerLogType && log.Type != internalLoadBalancerLogType {
		return fmt.Errorf("%w: %q, expected %q or %q", errUnexpectedLogType, log.Type, externalLoadBalancerLogType, internalLoadBalancerLogType)
	}

	handleTimestamps(log.StartTime, log.EndTime, attr)

	if err := handleConnection(log.Connection, attr); err != nil {
		return err
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBBytesReceived, log.BytesReceived, attr); err != nil {
		return fmt.Errorf("%w: %w", errBytesReceived, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBBytesSent, log.BytesSent, attr); err != nil {
		return fmt.Errorf("%w: %w", errBytesSent, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBPacketsReceived, log.PacketsReceived, attr); err != nil {
		return fmt.Errorf("%w: %w", errPacketsReceived, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBPacketsSent, log.PacketsSent, attr); err != nil {
		return fmt.Errorf("%w: %w", errPacketsSent, err)
	}

	if err := shared.PutDurationAsSeconds(gcpPassthroughNLBRTT, log.RTT, attr); err != nil {
		return fmt.Errorf("%w: %w", errRTT, err)
	}

	return nil
}

func handleConnection(conn *connection, attr pcommon.Map) error {
	if conn == nil {
		return nil
	}

	shared.PutStr(string(conventions.ClientAddressKey), conn.ClientIP, attr)
	shared.PutInt(string(conventions.ClientPortKey), conn.ClientPort, attr)
	shared.PutInt(string(conventions.ServerPortKey), conn.ServerPort, attr)

	if conn.Protocol != nil {
		if protoName, ok := shared.ProtocolName(uint32(*conn.Protocol)); ok {
			attr.PutStr(string(conventions.NetworkTransportKey), protoName)
		}
	}

	if _, err := shared.PutStrIfNotPresent(string(conventions.ServerAddressKey), conn.ServerIP, attr); err != nil {
		return fmt.Errorf("%w: %w", errServerAddress, err)
	}

	return nil
}

func handleTimestamps(start, end *time.Time, attr pcommon.Map) {
	if start != nil {
		attr.PutStr(gcpPassthroughNLBPacketsStartTime, start.Format(time.RFC3339Nano))
	}

	if end != nil {
		attr.PutStr(gcpPassthroughNLBPacketsEndTime, end.Format(time.RFC3339Nano))
	}
}
