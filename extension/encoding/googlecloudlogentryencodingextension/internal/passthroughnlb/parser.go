// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package passthroughnlb contains utilities for parsing Google Cloud Passthrough External and Internal Network Load Balancer logs.
package passthroughnlb // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/externalnlb"

import (
	"errors"
	"fmt"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"
)

const (
	// ConnectionsLogNameSuffix identifies load balancer connection logs in the logName field.
	ConnectionsLogNameSuffix = "loadbalancing.googleapis.com%2Fflows"

	externalLoadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.ExternalNetworkLoadBalancerLogEntry"
	internalLoadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.InternalNetworkLoadBalancerLogEntry"

	gcpPassthroughNLBPacketsStartTime = "gcp.load_balancing.passthrough_nlb.packets.start_time"
	gcpPassthroughNLBPacketsEndTime   = "gcp.load_balancing.passthrough_nlb.packets.end_time"
	gcpPassthroughNLBBytesReceived    = "gcp.load_balancing.passthrough_nlb.bytes_received"
	gcpPassthroughNLBBytesSent        = "gcp.load_balancing.passthrough_nlb.bytes_sent"
	gcpPassthroughNLBPacketsReceived  = "gcp.load_balancing.passthrough_nlb.packets_received"
	gcpPassthroughNLBPacketsSent      = "gcp.load_balancing.passthrough_nlb.packets_sent"
	gcpPassthroughNLBRTT              = "gcp.load_balancing.passthrough_nlb.rtt"
)

var (
	ErrUnmarshalPayload  = errors.New("failed to unmarshal Passthrough NLB log payload")
	ErrUnexpectedLogType = errors.New("unexpected log type")
	ErrBytesReceived     = errors.New("failed to add bytes received")
	ErrBytesSent         = errors.New("failed to add bytes sent")
	ErrPacketsReceived   = errors.New("failed to packets received")
	ErrPacketsSent       = errors.New("failed to add packets sent")
	ErrRTT               = errors.New("failed to add RTT")
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
		return fmt.Errorf("%w: %w", ErrUnmarshalPayload, err)
	}

	if log.Type != externalLoadBalancerLogType && log.Type != internalLoadBalancerLogType {
		return fmt.Errorf("%w: %q, expected %q or %q", ErrUnexpectedLogType, log.Type, externalLoadBalancerLogType, internalLoadBalancerLogType)
	}

	handleConnection(log.Connection, attr)
	handleTimestamps(log.StartTime, log.EndTime, attr)

	if err := shared.AddStrAsInt(gcpPassthroughNLBBytesReceived, log.BytesReceived, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrBytesReceived, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBBytesSent, log.BytesSent, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrBytesSent, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBPacketsReceived, log.PacketsReceived, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrPacketsReceived, err)
	}

	if err := shared.AddStrAsInt(gcpPassthroughNLBPacketsSent, log.PacketsSent, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrPacketsSent, err)
	}

	if err := shared.PutDurationAsSeconds(gcpPassthroughNLBRTT, log.RTT, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrRTT, err)
	}

	return nil
}

func handleConnection(conn *connection, attr pcommon.Map) {
	if conn == nil {
		return
	}

	shared.PutStr(string(semconv.ClientAddressKey), conn.ClientIP, attr)
	shared.PutInt(string(semconv.ClientPortKey), conn.ClientPort, attr)

	shared.PutStr(string(semconv.ServerAddressKey), conn.ServerIP, attr)
	shared.PutInt(string(semconv.ServerPortKey), conn.ServerPort, attr)

	if conn.Protocol != nil {
		if protoName, ok := vpcflowlog.ProtocolName(uint32(*conn.Protocol)); ok {
			attr.PutStr(string(semconv.NetworkTransportKey), protoName)
		}
	}
}

func handleTimestamps(start, end *time.Time, attr pcommon.Map) {
	if start != nil {
		attr.PutStr(gcpPassthroughNLBPacketsStartTime, start.Format(time.RFC3339Nano))
	}

	if end != nil {
		attr.PutStr(gcpPassthroughNLBPacketsEndTime, end.Format(time.RFC3339Nano))
	}
}
