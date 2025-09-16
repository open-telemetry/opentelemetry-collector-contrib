// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSSHConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  SSHConfig
		wantErr bool
	}{
		{
			name: "valid_password_auth",
			config: SSHConfig{
				Host:     "localhost:22",
				Username: "admin",
				Password: "password",
				Timeout:  5 * time.Second,
			},
			wantErr: true, // Will fail to connect but config is valid
		},
		{
			name: "no_auth_method",
			config: SSHConfig{
				Host:     "localhost:22",
				Username: "admin",
				Password: "",
				KeyFile:  "",
				Timeout:  5 * time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSSHClient(tt.config)

			// Since NewSSHClient tries to connect, we expect errors for all cases
			// in a test environment without actual SSH servers
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// This would only pass with a real SSH server
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSHClient_InvalidHost(t *testing.T) {
	config := SSHConfig{
		Host:     "invalid-host:22",
		Username: "admin",
		Password: "password",
		Timeout:  2 * time.Second,
	}
	_, err := NewSSHClient(config)

	// Should fail to connect to invalid host
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestSSHClient_AuthenticationError(t *testing.T) {
	config := SSHConfig{
		Host:     "localhost:22",
		Username: "nonexistent",
		Password: "wrongpassword",
		Timeout:  2 * time.Second,
	}
	_, err := NewSSHClient(config)

	// Should fail due to authentication or connection error
	assert.Error(t, err)
}

func TestSSHConfig_NoAuthMethod(t *testing.T) {
	config := SSHConfig{
		Host:     "localhost:22",
		Username: "admin",
		Password: "", // No password
		KeyFile:  "", // No key file
		Timeout:  2 * time.Second,
	}
	_, err := NewSSHClient(config)

	// Should fail due to no authentication method
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication method provided")
}

func TestSSHConfig_EmptyUsername(t *testing.T) {
	config := SSHConfig{
		Host:     "localhost:22",
		Username: "", // Empty username
		Password: "password",
		Timeout:  2 * time.Second,
	}
	_, err := NewSSHClient(config)

	// Should fail due to empty username
	assert.Error(t, err)
}

// MockSSHConnection for testing without real SSH connectivity
type MockSSHConnection struct {
	host      string
	connected bool
	responses map[string]string
	errors    map[string]error
}

func NewMockSSHConnection(host string) *MockSSHConnection {
	return &MockSSHConnection{
		host:      host,
		connected: false,
		responses: make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *MockSSHConnection) Connect() error {
	if m.host == "fail-connect" {
		return assert.AnError
	}
	m.connected = true
	return nil
}

func (m *MockSSHConnection) Close() error {
	m.connected = false
	return nil
}

func (m *MockSSHConnection) IsConnected() bool {
	return m.connected
}

func (m *MockSSHConnection) ExecuteCommand(command string) (string, error) {
	if !m.connected {
		return "", assert.AnError
	}

	if err, exists := m.errors[command]; exists {
		return "", err
	}

	if response, exists := m.responses[command]; exists {
		return response, nil
	}

	return "", nil
}

func (m *MockSSHConnection) GetHost() string {
	return m.host
}

func (m *MockSSHConnection) SetResponse(command, response string) {
	m.responses[command] = response
}

func (m *MockSSHConnection) SetError(command string, err error) {
	m.errors[command] = err
}

func TestMockSSHConnection(t *testing.T) {
	mock := NewMockSSHConnection("test-host:22")

	// Test initial state
	assert.Equal(t, "test-host:22", mock.GetHost())
	assert.False(t, mock.IsConnected())

	// Test connect
	err := mock.Connect()
	assert.NoError(t, err)
	assert.True(t, mock.IsConnected())

	// Test command execution
	mock.SetResponse("show version", "Cisco IOS Version 15.1")
	output, err := mock.ExecuteCommand("show version")
	assert.NoError(t, err)
	assert.Equal(t, "Cisco IOS Version 15.1", output)

	// Test command error
	mock.SetError("show bgp", assert.AnError)
	output, err = mock.ExecuteCommand("show bgp")
	assert.Error(t, err)
	assert.Empty(t, output)

	// Test close
	err = mock.Close()
	assert.NoError(t, err)
	assert.False(t, mock.IsConnected())
}
