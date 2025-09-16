// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
)

// OSType represents the operating system type of a Cisco device
type OSType string

const (
	IOSXE OSType = "IOS XE"
	NXOS  OSType = "NX-OS"
	IOS   OSType = "IOS"
)

// Client represents an RPC client for Cisco devices
type Client struct {
	ssh    *connection.SSHClient
	osType OSType
	target string
}

// NewClient creates a new RPC client and detects the OS type
func NewClient(sshClient *connection.SSHClient) (*Client, error) {
	client := &Client{
		ssh:    sshClient,
		target: sshClient.Target(),
	}

	// Detect OS type
	osType, err := client.detectOSType()
	if err != nil {
		return nil, fmt.Errorf("failed to detect OS type: %w", err)
	}

	client.osType = osType
	return client, nil
}

// detectOSType detects the operating system type of the device
func (c *Client) detectOSType() (OSType, error) {
	output, err := c.ssh.ExecuteCommand("show version")
	if err != nil {
		return "", fmt.Errorf("failed to execute show version: %w", err)
	}

	// Check for IOS XE
	if strings.Contains(output, "Cisco IOS XE") {
		return IOSXE, nil
	}

	// Check for NX-OS
	if strings.Contains(output, "Cisco Nexus") || strings.Contains(output, "NX-OS") {
		return NXOS, nil
	}

	// Check for IOS
	if strings.Contains(output, "Cisco IOS Software") {
		return IOS, nil
	}

	// Default to IOS XE if uncertain
	return IOSXE, nil
}

// ExecuteCommand executes a command on the device
func (c *Client) ExecuteCommand(command string) (string, error) {
	return c.ssh.ExecuteCommand(command)
}

// GetOSType returns the detected OS type
func (c *Client) GetOSType() OSType {
	return c.osType
}

// GetTarget returns the target address
func (c *Client) GetTarget() string {
	return c.target
}

// IsOSSupported checks if the OS type supports a specific feature
func (c *Client) IsOSSupported(feature string) bool {
	switch feature {
	case "bgp":
		return c.osType == IOSXE || c.osType == NXOS
	case "environment":
		return true // All OS types support environment commands
	case "facts":
		return true // All OS types support facts commands
	case "interfaces":
		return true // All OS types support interface commands
	case "optics":
		return c.osType == IOSXE || c.osType == NXOS
	default:
		return false
	}
}

// GetCommand returns the appropriate command for the OS type and feature
func (c *Client) GetCommand(feature string) string {
	switch feature {
	case "bgp":
		if c.osType == NXOS {
			return "show bgp all summary"
		}
		// For IOS/IOS XE, try more compatible commands
		return "show ip bgp summary"
	case "environment":
		return "show environment"
	case "facts_version":
		return "show version"
	case "facts_memory":
		if c.osType == NXOS {
			return "show system resources"
		}
		return "show memory statistics"
	case "facts_cpu":
		if c.osType == NXOS {
			return "show system resources"
		}
		return "show processes cpu"
	case "interfaces":
		if c.osType == NXOS {
			return "show interface"
		}
		// For IOS/IOS XE, use standard command
		return "show interfaces"
	case "interfaces_vlans":
		if c.osType == IOSXE {
			return "show vlans"
		}
		return ""
	case "optics":
		if c.osType == NXOS {
			return "show interface transceiver"
		}
		return "show interfaces transceiver"
	default:
		return ""
	}
}

// Close closes the underlying SSH connection
func (c *Client) Close() error {
	return c.ssh.Close()
}
