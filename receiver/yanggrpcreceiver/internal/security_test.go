// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(1.0, 2, time.Minute) // 1 request per second, burst of 2
	defer rl.Stop()

	// First request should be allowed (from burst)
	assert.True(t, rl.Allow("192.168.1.1"))

	// Second request should be allowed (from burst)
	assert.True(t, rl.Allow("192.168.1.1"))

	// Third request should be denied (rate limited - burst exhausted)
	assert.False(t, rl.Allow("192.168.1.1"))

	// Different IP should be allowed (has its own bucket)
	assert.True(t, rl.Allow("192.168.1.2"))
}

func TestSecurityManager_ClientAuthTypes(t *testing.T) {
	sm := &SecurityManager{}

	tests := []struct {
		authType string
		expected tls.ClientAuthType
		hasError bool
	}{
		{"NoClientCert", tls.NoClientCert, false},
		{"RequestClientCert", tls.RequestClientCert, false},
		{"RequireAnyClientCert", tls.RequireAnyClientCert, false},
		{"VerifyClientCertIfGiven", tls.VerifyClientCertIfGiven, false},
		{"RequireAndVerifyClientCert", tls.RequireAndVerifyClientCert, false},
		{"InvalidAuthType", tls.NoClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.authType, func(t *testing.T) {
			authType, err := sm.getClientAuthType(tt.authType)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, authType)
			}
		})
	}
}
