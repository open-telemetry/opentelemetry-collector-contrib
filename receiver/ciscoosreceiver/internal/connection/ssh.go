// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient represents an SSH connection to a Cisco device
type SSHClient struct {
	client *ssh.Client
	config *ssh.ClientConfig
	target string
}

// SSHConfig holds SSH connection configuration
type SSHConfig struct {
	Host     string
	Username string
	Password string
	KeyFile  string
	Timeout  time.Duration
}

// NewSSHClient creates a new SSH client connection
func NewSSHClient(config SSHConfig) (*SSHClient, error) {
	var authMethods []ssh.AuthMethod

	// Determine authentication method based on configuration
	// Method 1: SSH key file authentication (preferred)
	if config.KeyFile != "" {
		keyAuth, err := createKeyAuth(config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH key authentication: %w", err)
		}
		authMethods = append(authMethods, keyAuth)
	} else if config.Password != "" && config.Username != "" {
		// Method 2: Username/password authentication
		authMethods = append(authMethods, ssh.Password(config.Password))
	} else {
		return nil, errors.New("no authentication method provided: either key_file (Method 1) or username+password (Method 2) is required")
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         config.Timeout,
	}

	conn, err := ssh.Dial("tcp", config.Host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", config.Host, err)
	}

	return &SSHClient{
		client: conn,
		config: sshConfig,
		target: config.Host,
	}, nil
}

// createKeyAuth creates SSH key authentication from key file
func createKeyAuth(keyFile string) (ssh.AuthMethod, error) {
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key file %s: %w", keyFile, err)
	}

	// Parse private key without passphrase
	privateKey, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	return ssh.PublicKeys(privateKey), nil
}

// ExecuteCommand executes a command on the remote device
func (c *SSHClient) ExecuteCommand(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	return string(output), nil
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// IsConnected checks if the SSH connection is still active
func (c *SSHClient) IsConnected() bool {
	if c.client == nil {
		return false
	}

	// Try to create a session to test connectivity
	session, err := c.client.NewSession()
	if err != nil {
		return false
	}
	session.Close()
	return true
}

// Target returns the target address
func (c *SSHClient) Target() string {
	return c.target
}
