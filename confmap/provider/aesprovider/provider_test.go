// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aesprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestAESCredentialProvider(t *testing.T) {
	tests := []struct {
		envVars       map[string]string
		name          string
		configValue   string
		expectedValue string
		expectedError string
	}{
		{
			name:          "Valid type, key, JSON value",
			configValue:   `aes:RsEf6cTWrssi8tls+5Tlk3H8AQBVT2uVDcJ88zj3Sph1iSN8z97yGlxNrWf7oUHSyS4lISEcuqrQq2sPsvcl480rEajM0DIWNVoPi2ApKhADCygGoRxbykm4Tuce+rx0aWIPj1zDkuBM6NIYI1E9H2gxrH+P89Hlwmwq26S3HkqIvtW/BFAHJ8Z08iIg95NFwdbZBu4bMwY93r0KWhL0Y4xk9PGmCZhHk5jaaCgd5YREhJCH8ahenZ/t71yGdu71gSIVbg1xwGZNDKf9wRpOCW4oVQ+OWORDrgKX+58QZMOu0zjRxIkeFQV66xMIEziLYERW0xnT/GthH2+FdO/OlrUlzTBnQBbgqpp86cW1xKPFE1XeFJAIHXca7sYIjl/bz6csiUGSXKVyzApsz6fQEhaFwPGw7YKg/hN4+AEu7zYwHlpiPZyQZVE8xPEgwAy1ZKKk7nJ391ujXohXEsmc7u40bOdQmvktyjemf3KcYSCSoVqvdm49K8/ZfKgG1LMCSnHMk0HWdVGyO8jjBj3RnhpT1/dQ/aMudwPMsPFE+85GYEPTe9Sjq8erkjLRfGomDYWJhSpMESMovVRKpyjv9W+3xS0fHracj1AIx8iRQ9KNq7YKSX9n+wtE7LXMr6CUxwDs0wm1FCdVpc/JfH7BghbAh9qNSZg7qeNWQO9BG9vVa31EpRTDHOOogzGOQU2APtjc0qU65we6lpShBl2HU6S/5SgjB5m9ZnCsJSqlCQTf/e/Riuvx8l5LBlv1JNbHnLD6LO7xarpEKzR2Nc2N2+6pP86SvVB/ZqxGug06SUckjQbrmVrjU5X0RFWQAb4ZdPUobxk2xOXGhxUxEB/pDv5DcuDaEry97XsYBgzYpCtVZr8uQc5kd5jPcMsVgIYo78t+v+2yvCdYtRSHOrAcrOyBbrXCo1yI4UA9qAmfBE1PWC7km9xdhtlIAA5Szei+2oRxCwSvVO0TeYCwByDmYDolL0Tv5jtdgsPbcgnZsL/b9KRBAUU4wXKVm55mzw3AiOehX/bms84XLnRWZaxN06tJ/DiMbMcatTQP0pxk4zoemVD66wo7dA8U0nrnfP8AMfQmFQ==`,
			expectedValue: `{   "type": "service_account",   "project_id": "my-test-project-12345",   "private_key_id": "abcdef1234567890abcdef1234567890abcdef12",   "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCAmMwggJfAgEAAoGBALKlO+j9ALhlg5po\nfakePrivateKeyValue+/abc/def/123/fakeData==\n-----END PRIVATE KEY-----\n",   "client_email": "test-service-account@my-test-project-12345.iam.gserviceaccount.com",   "client_id": "123456789012345678901",   "auth_uri": "https://accounts.google.com/o/oauth2/auth",   "token_uri": "https://oauth2.googleapis.com/token",   "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",   "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-service-account%40my-test-project-12345.iam.gserviceaccount.com" }`,

			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac=",
			},
		},

		{
			name:          "Invalid base64 key",
			configValue:   `aes:RsEf6cTWrssi8tlssfs1AJs2bRMrVm2Ce5TaWPY=`,
			expectedError: `"aes" provider uri failed to base64 decode key: illegal base64 data at input byte 25`,

			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8lN8bVU2k0TAKtzwJzac=",
			},
		},
		{
			name:          "Invalid AES key",
			configValue:   `aes:RsEf6cTWrssi8tls+5Tlk3H8AQBVT2uVDcJ88zj3Sph1iSN8z97yGlxNrWf7oUHSyS4lISEcuqrQq2sPsvcl480rEajM0DIWNVoPi2ApKhADCygGoRxbykm4Tuce+rx0aWIPj1zDkuBM6NIYI1E9H2gxrH+P89Hlwmwq26S3HkqIvtW/BFAHJ8Z08iIg95NFwdbZBu4bMwY93r0KWhL0Y4xk9PGmCZhHk5jaaCgd5YREhJCH8ahenZ/t71yGdu71gSIVbg1xwGZNDKf9wRpOCW4oVQ+OWORDrgKX+58QZMOu0zjRxIkeFQV66xMIEziLYERW0xnT/GthH2+FdO/OlrUlzTBnQBbgqpp86cW1xKPFE1XeFJAIHXca7sYIjl/bz6csiUGSXKVyzApsz6fQEhaFwPGw7YKg/hN4+AEu7zYwHlpiPZyQZVE8xPEgwAy1ZKKk7nJ391ujXohXEsmc7u40bOdQmvktyjemf3KcYSCSoVqvdm49K8/ZfKgG1LMCSnHMk0HWdVGyO8jjBj3RnhpT1/dQ/aMudwPMsPFE+85GYEPTe9Sjq8erkjLRfGomDYWJhSpMESMovVRKpyjv9W+3xS0fHracj1AIx8iRQ9KNq7YKSX9n+wtE7LXMr6CUxwDs0wm1FCdVpc/JfH7BghbAh9qNSZg7qeNWQO9BG9vVa31EpRTDHOOogzGOQU2APtjc0qU65we6lpShBl2HU6S/5SgjB5m9ZnCsJSqlCQTf/e/Riuvx8l5LBlv1JNbHnLD6LO7xarpEKzR2Nc2N2+6pP86SvVB/ZqxGug06SUckjQbrmVrjU5X0RFWQAb4ZdPUobxk2xOXGhxUxEB/pDv5DcuDaEry97XsYBgzYpCtVZr8uQc5kd5jPcMsVgIYo78t+v+2yvCdYtRSHOrAcrOyBbrXCo1yI4UA9qAmfBE1PWC7km9xdhtlIAA5Szei+2oRxCwSvVO0TeYCwByDmYDolL0Tv5jtdgsPbcgnZsL/b9KRBAUU4wXKVm55mzw3AiOehX/bms84XLnRWZaxN06tJ/DiMbMcatTQP0pxk4zoemVD66wo7dA8U0nrnfP8AMfQmFQ==`,
			expectedError: `"aes" provider failed to decrypt value: crypto/aes: invalid key size 1`,

			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "MQ==",
			},
		},

		{
			name:          "simple message",
			configValue:   "aes:RsEf6cTWrssi8tlssfs1AJs2bRMrVm2Ce5TaWPY=",
			expectedValue: "1",
			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac=",
			},
		},
		{
			name:          "empty message",
			configValue:   "aes:RsEf6cTWrssi8tlsZ4V68iFEJRFI8o71+QoYYw==",
			expectedValue: "",
			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac=",
			},
		},
		{
			name:          "Truncated base64 value",
			configValue:   "aes:R=",
			expectedError: `"aes" provider failed to decrypt value: illegal base64 data at input byte 1`,
			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac=",
			},
		},
		{
			name:          "Truncated encryted text",
			configValue:   "aes:MQ==",
			expectedError: `"aes" provider failed to decrypt value: ciphertext too short`,
			envVars: map[string]string{
				"OTEL_AES_CREDENTIAL_PROVIDER": "GQi+Y8HwOYzs8lAOjHUqB7vXlN8bVU2k0TAKtzwJzac=",
			},
		},
		{
			name:          "Wrong schema",
			configValue:   "foo:MQ==",
			expectedError: `"foo:MQ==" uri is not supported by "aes" provider`,
		},
		{
			name:          "No env vars",
			configValue:   "aes:MQ==",
			expectedError: `env var "OTEL_AES_CREDENTIAL_PROVIDER" not set, required for "aes" provider`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			p := NewFactory().Create(confmap.ProviderSettings{})
			retrieved, err := p.Retrieve(context.Background(), tt.configValue, nil)
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err.Error())
				return
			}
			require.NoError(t, err)
			require.NotNil(t, retrieved)
			stringValue, err := retrieved.AsString()
			require.NoError(t, err)
			require.Equal(t, tt.expectedValue, stringValue)
		})
	}
}
