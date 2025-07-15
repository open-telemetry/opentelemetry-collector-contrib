// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var _ dockerProvider = (*mockDockerProvider)(nil)

type dockerProvider interface {
	Hostname(context.Context) (string, error)
}

type mockDockerProvider struct {
	hostname string
	err      error
}

func (m *mockDockerProvider) Hostname(context.Context) (string, error) {
	return m.hostname, m.err
}

func (m *mockDockerProvider) OSType(context.Context) (string, error) {
	return "linux", nil
}

func TestProvider(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		err          error
		expectedSrc  source.Source
		expectError  bool
		errorMessage string
	}{
		{
			name:        "successful hostname retrieval",
			hostname:    "docker-host",
			err:         nil,
			expectedSrc: source.Source{Kind: source.HostnameKind, Identifier: "docker-host"},
			expectError: false,
		},
		{
			name:         "empty hostname",
			hostname:     "",
			err:          nil,
			expectError:  true,
			errorMessage: "Docker hostname is empty",
		},
		{
			name:         "docker error",
			hostname:     "",
			err:          errors.New("docker connection failed"),
			expectError:  true,
			errorMessage: "failed to get Docker hostname: docker connection failed",
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider := &Provider{
				logger: zap.NewNop(),
				detector: &mockDockerProvider{
					hostname: testInstance.hostname,
					err:      testInstance.err,
				},
			}

			src, err := provider.Source(context.Background())
			if testInstance.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), testInstance.errorMessage)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testInstance.expectedSrc, src)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	logger := zap.NewNop()

	// Test that NewProvider doesn't fail (it should try to connect to Docker)
	// This test will pass if Docker is available, or gracefully handle the error if it's not
	provider, err := NewProvider(logger)

	// We don't assert no error here because Docker might not be available in CI
	// But we do test that if provider is created, it's properly initialized
	if err == nil {
		require.NotNil(t, provider)
		assert.NotNil(t, provider.logger)
		assert.NotNil(t, provider.detector)
	}
}
