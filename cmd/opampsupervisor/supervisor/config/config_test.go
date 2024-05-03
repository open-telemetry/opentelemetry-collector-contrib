// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name          string
		config        Supervisor
		expectedError string
	}{
		{
			name: "Valid filled out config",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "${executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "Endpoint unspecified",
			config: Supervisor{
				Server: OpAMPServer{
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "${executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "server::endpoint must be specified",
		},
		{
			name: "Invalid URL",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "\000",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "${executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "invalid URL for server::endpoint:",
		},
		{
			name: "Invalid endpoint scheme",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "tcp://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "${executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: `invalid scheme "tcp" for server::endpoint, must be one of "http", "https", "ws", or "wss"`,
		},
		{
			name: "Invalid tls settings",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
						Config: configtls.Config{
							MaxVersion: "1.2",
							MinVersion: "1.3",
						},
					},
				},
				Agent: Agent{
					Executable: "${executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "invalid server::tls settings:",
		},
		{
			name: "Empty agent executable path",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::executable must be specified",
		},
		{
			name: "agent executable does not exist",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "./path/does/not/exist",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "could not stat agent::executable path:",
		},
		{
			name: "agent executable has no exec bits set",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: Agent{
					Executable: "${non_executable_path}",
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::executable does not have executable bit set",
		},
	}

	// create some fake files for validating agent config
	tmpDir := t.TempDir()

	executablePath := filepath.Join(tmpDir, "agent.exe")
	//#nosec G306 -- need to write executable file for test
	require.NoError(t, os.WriteFile(executablePath, []byte{}, 0100))

	nonExecutablePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(nonExecutablePath, []byte{}, 0600))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill in path to agent executable
			tc.config.Agent.Executable = os.Expand(tc.config.Agent.Executable,
				func(s string) string {
					switch s {
					case "executable_path":
						return executablePath
					case "non_executable_path":
						return nonExecutablePath
					}

					return ""
				})

			err := tc.config.Validate()

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}
