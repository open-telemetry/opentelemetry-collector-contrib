// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

// Client represents SSH client connection to Cisco device
type Client struct {
	Target     string
	Username   string
	Connection *cryptossh.Client
	Logger     *zap.Logger
}

// DetectOSType executes "show version" to detect Cisco OS type
func (s *Client) DetectOSType(ctx context.Context) (string, error) {
	s.Logger.Debug("Executing 'show version' command for OS detection")

	output, err := s.ExecuteCommand(ctx, "show version")
	if err != nil {
		return "", fmt.Errorf("failed to execute 'show version': %w", err)
	}

	s.Logger.Debug("Analyzing show version output", zap.Int("output_length", len(output)))

	// OS detection from show version output
	if strings.Contains(output, "Cisco IOS XE") {
		return "IOS XE", nil
	}
	if strings.Contains(output, "Cisco Nexus") || strings.Contains(output, "NX-OS") {
		return "NX-OS", nil
	}
	if strings.Contains(output, "Cisco IOS Software") {
		return "IOS", nil
	}

	// Default to IOS XE if uncertain
	s.Logger.Warn("Unable to detect OS type from show version output, defaulting to IOS XE")
	return "IOS XE", nil
}

// ExecuteCommand executes a command on the Cisco device via SSH
func (s *Client) ExecuteCommand(ctx context.Context, command string) (string, error) {
	s.Logger.Debug("Executing SSH command", zap.String("command", command))

	// Create SSH session
	session, err := s.Connection.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Configure terminal modes for Cisco devices
	modes := cryptossh.TerminalModes{
		cryptossh.ECHO:          0,     // disable echoing
		cryptossh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		cryptossh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	// Request pseudo terminal (required for interactive Cisco CLI)
	if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
		s.Logger.Warn("Failed to request pseudo terminal, continuing anyway", zap.Error(err))
	}

	// Execute command with context timeout
	resultChan := make(chan struct {
		output string
		err    error
	}, 1)

	go func() {
		// Send command followed by newline
		output, err := session.CombinedOutput(command + "\n")
		resultChan <- struct {
			output string
			err    error
		}{string(output), err}
	}()

	select {
	case result := <-resultChan:
		if result.err != nil {
			return "", fmt.Errorf("command execution failed: %w", result.err)
		}
		s.Logger.Debug("Command executed successfully",
			zap.String("command", command),
			zap.Int("output_length", len(result.output)))
		return result.output, nil

	case <-ctx.Done():
		return "", fmt.Errorf("command execution timeout: %w", ctx.Err())
	}
}

// Close closes SSH connection
func (s *Client) Close() error {
	if s.Connection != nil {
		return s.Connection.Close()
	}
	return nil
}
