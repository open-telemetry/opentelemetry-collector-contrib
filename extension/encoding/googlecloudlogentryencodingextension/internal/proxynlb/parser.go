// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package proxynlb contains utilities for parsing Google Cloud Proxy Network Load Balancer logs.
package proxynlb // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/proxynlb"

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
	ConnectionsLogNameSuffix = "loadbalancing.googleapis.com%2Fconnections"

	loadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry"

	gcpProxyNLBConnectionStartTime = "gcp.load_balancing.proxy_nlb.connection.start_time"
	gcpProxyNLBConnectionEndTime   = "gcp.load_balancing.proxy_nlb.connection.end_time"
	gcpProxyNLBServerBytesReceived = "gcp.load_balancing.proxy_nlb.server.bytes_received"
	gcpProxyNLBServerBytesSent     = "gcp.load_balancing.proxy_nlb.server.bytes_sent"
)

var (
	ErrUnmarshalPayload    = errors.New("failed to unmarshal Proxy NLB log payload")
	ErrUnexpectedLogType   = errors.New("unexpected log type")
	ErrServerBytesReceived = errors.New("failed to add server bytes received")
	ErrServerBytesSent     = errors.New("failed to add server bytes sent")
)

type loadBalancerLog struct {
	Type       string      `json:"@type"`
	Connection *connection `json:"connection"`
	StartTime  *time.Time  `json:"startTime"`
	EndTime    *time.Time  `json:"endTime"`

	// ServerBytesReceived and ServerBytesSent are string-encoded 64-bit integers.
	// Although the official documentation (https://docs.cloud.google.com/load-balancing/docs/tcp/tcp-ssl-proxy-logging-monitoring)
	// specifies these fields as integers, the actual
	// JSON payload from Cloud Logging seems to stringify int64/uint64 values to
	// preserve precision
	ServerBytesReceived string `json:"serverBytesReceived"`
	ServerBytesSent     string `json:"serverBytesSent"`
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

	if log.Type != loadBalancerLogType {
		return fmt.Errorf("%w: %q, expected %q", ErrUnexpectedLogType, log.Type, loadBalancerLogType)
	}

	handleConnection(log.Connection, attr)
	handleTimestamps(log.StartTime, log.EndTime, attr)

	if err := shared.AddStrAsInt(gcpProxyNLBServerBytesReceived, log.ServerBytesReceived, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrServerBytesReceived, err)
	}

	if err := shared.AddStrAsInt(gcpProxyNLBServerBytesSent, log.ServerBytesSent, attr); err != nil {
		return fmt.Errorf("%w: %w", ErrServerBytesSent, err)
	}

	return nil
}

func handleConnection(conn *connection, attr pcommon.Map) {
	if conn == nil {
		return
	}

	shared.PutStr(string(conventions.ClientAddressKey), conn.ClientIP, attr)
	shared.PutInt(string(conventions.ClientPortKey), conn.ClientPort, attr)

	shared.PutStr(string(conventions.ServerAddressKey), conn.ServerIP, attr)
	shared.PutInt(string(conventions.ServerPortKey), conn.ServerPort, attr)

	if conn.Protocol != nil {
		if protoName, ok := shared.ProtocolName(uint32(*conn.Protocol)); ok {
			attr.PutStr(string(conventions.NetworkTransportKey), protoName)
		}
	}
}

func handleTimestamps(start, end *time.Time, attr pcommon.Map) {
	if start != nil {
		attr.PutStr(gcpProxyNLBConnectionStartTime, start.Format(time.RFC3339Nano))
	}

	if end != nil {
		attr.PutStr(gcpProxyNLBConnectionEndTime, end.Format(time.RFC3339Nano))
	}
}
