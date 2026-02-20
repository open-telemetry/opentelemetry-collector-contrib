// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				AllowAllKeys:       false,
				AllowedKeys:        []string{"description", "group", "id", "name"},
				IgnoredKeys:        []string{"safe_attribute"},
				BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?", "(5[1-5][0-9]{14})"},
				BlockedKeyPatterns: []string{".*(token|api_key).*"},
				HashFunction:       MD5,
				AllowedValues:      []string{".+@mycompany.com"},
				Summary:            debug,
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: false,
					},
					RedisConfig: db.RedisConfig{
						Enabled: false,
					},
					MongoConfig: db.MongoConfig{
						Enabled: false,
					},
				},
			},
		},
		{
			id:       component.NewIDWithName(metadata.Type, "empty"),
			expected: createDefaultConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name     string
		hash     HashFunction
		expected error
	}{
		{
			name: "valid",
			hash: MD5,
		},
		{
			name: "empty",
			hash: None,
		},
		{
			name:     "invalid",
			hash:     "hash",
			expected: errors.New("unknown HashFunction hash, allowed functions are sha1, sha3 and md5"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h HashFunction
			err := h.UnmarshalText([]byte(tt.hash))
			if tt.expected != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.hash, h)
			}
		})
	}
}

func TestValidateHMACKey(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectError   bool
		errorContains string
	}{
		{
			name: "valid HMAC-SHA256 with sufficient key length",
			config: &Config{
				HashFunction: HMACSHA256,
				HMACKey:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 32 bytes
			},
			expectError: false,
		},
		{
			name: "valid HMAC-SHA512 with sufficient key length",
			config: &Config{
				HashFunction: HMACSHA512,
				HMACKey:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", // 64 bytes
			},
			expectError: false,
		},
		{
			name: "empty key with HMAC-SHA256",
			config: &Config{
				HashFunction: HMACSHA256,
				HMACKey:      "",
			},
			expectError:   true,
			errorContains: "hmac_key must not be empty",
		},
		{
			name: "empty key with HMAC-SHA512",
			config: &Config{
				HashFunction: HMACSHA512,
				HMACKey:      "",
			},
			expectError:   true,
			errorContains: "hmac_key must not be empty",
		},
		{
			name: "key too short for HMAC-SHA256",
			config: &Config{
				HashFunction: HMACSHA256,
				HMACKey:      "short-key",
			},
			expectError:   true,
			errorContains: "hmac_key must be at least 32 bytes long",
		},
		{
			name: "key too short for HMAC-SHA512",
			config: &Config{
				HashFunction: HMACSHA512,
				HMACKey:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 32 bytes, too short for SHA512
			},
			expectError:   true,
			errorContains: "hmac_key must be at least 64 bytes long",
		},
		{
			name: "no validation for non-HMAC hash functions",
			config: &Config{
				HashFunction: MD5,
				HMACKey:      "",
			},
			expectError: false,
		},
		{
			name: "no validation when hash function is None",
			config: &Config{
				HashFunction: None,
				HMACKey:      "",
			},
			expectError: false,
		},
		{
			name: "key with special characters is allowed",
			config: &Config{
				HashFunction: HMACSHA256,
				HMACKey:      "!@#$%^&*()_+-=[]{}|;:,.<>?",
			},
			expectError:   true,
			errorContains: "hmac_key must be at least 32 bytes long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
