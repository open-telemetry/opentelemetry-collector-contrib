// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RPCClient represents RPC client for executing Cisco commands
type RPCClient struct {
	SSHClient *Client
	OSType    string
	Logger    *zap.Logger
}

// GetOSType returns detected Cisco OS type
func (r *RPCClient) GetOSType() string {
	if r.OSType != "" {
		return r.OSType
	}
	return "IOS XE" // Default
}

// GetCommand returns the appropriate command for the OS type and feature
func (r *RPCClient) GetCommand(feature string) string {
	switch feature {
	case "version":
		return "show version"
	case "cpu":
		if r.OSType == "NX-OS" {
			return "show system resources"
		}
		return "show process cpu"
	case "memory":
		if r.OSType == "NX-OS" {
			return "show system resources"
		}
		return "show process memory"
	case "interfaces":
		return "show interface"
	default:
		return ""
	}
}

// ExecuteCommand executes a command on the Cisco device
func (r *RPCClient) ExecuteCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return r.SSHClient.ExecuteCommand(ctx, command)
}
