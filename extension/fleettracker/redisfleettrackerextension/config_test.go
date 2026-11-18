// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisfleettrackerextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing endpoint",
			cfg:     Config{HeartbeatInterval: time.Second, TTL: 3 * time.Second},
			wantErr: "endpoint is required",
		},
		{
			name:    "zero heartbeat interval",
			cfg:     Config{Endpoint: "localhost:6379", HeartbeatInterval: 0, TTL: time.Second},
			wantErr: "heartbeat_interval must be greater than zero",
		},
		{
			name:    "ttl equal to heartbeat interval",
			cfg:     Config{Endpoint: "localhost:6379", HeartbeatInterval: time.Second, TTL: time.Second},
			wantErr: "ttl must be greater than heartbeat_interval",
		},
		{
			name:    "ttl less than heartbeat interval",
			cfg:     Config{Endpoint: "localhost:6379", HeartbeatInterval: 3 * time.Second, TTL: time.Second},
			wantErr: "ttl must be greater than heartbeat_interval",
		},
		{
			name: "valid",
			cfg:  Config{Endpoint: "localhost:6379", HeartbeatInterval: time.Second, TTL: 3 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, "fleet_tracker", cfg.KeyPrefix)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 10*time.Second, cfg.TTL)
	assert.True(t, cfg.TLS.Insecure)
}
