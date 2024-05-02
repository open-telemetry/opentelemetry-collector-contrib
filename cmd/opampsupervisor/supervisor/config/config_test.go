// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name          string
		config        Supervisor
		expectedError string
	}{
		{
			name:   "Empty Config is valid",
			config: Supervisor{},
		},
		{
			name: "Valid filled out config",
			config: Supervisor{
				Server: &OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
					},
				},
				Agent: &Agent{
					Executable: "../../otelcol",
					Description: AgentDescription{
						IdentifyingAttributes: map[string]string{
							"client.id": "some-client-id",
						},
						NonIdentifyingAttributes: map[string]string{
							"env": "dev",
						},
					},
				},
				Capabilities: &Capabilities{
					AcceptsRemoteConfig: asPtr(true),
				},
				Storage: &Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "Cannot override instance ID",
			config: Supervisor{
				Agent: &Agent{
					Executable: "../../otelcol",
					Description: AgentDescription{
						IdentifyingAttributes: map[string]string{
							semconv.AttributeServiceInstanceID: "instance-id",
						},
					},
				},
			},
			expectedError: `cannot override identifying attribute "service.instance.id"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}

func asPtr[T any](t T) *T {
	return &t
}
