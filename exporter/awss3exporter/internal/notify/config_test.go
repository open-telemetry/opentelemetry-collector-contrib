// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
)

func TestNewDefaultConfig(t *testing.T) {
	t.Parallel()

	c := NewDefaultConfig()

	assert.Equal(t, "", c.Endpoint, "feature must be off by default")
	assert.Equal(t, DefaultTimeout, c.Timeout)
	assert.Equal(t, DefaultQueueSize, c.QueueSize)
	assert.Equal(t, DefaultWorkers, c.Workers)
	assert.Equal(t, DefaultMaxRecordsPerPost, c.MaxRecordsPerPost)
	assert.Equal(t, DefaultMaxAttempts, c.MaxAttempts)
	assert.Equal(t, DefaultInitialBackoff, c.InitialBackoff)
	assert.Equal(t, DefaultMaxBackoff, c.MaxBackoff)
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	// valid: populated defaults with Endpoint set.
	valid := func() Config {
		c := NewDefaultConfig()
		c.Endpoint = "https://example.com/hook"
		return c
	}

	tests := []struct {
		name        string
		mutate      func(*Config)
		wantErr     bool
		errContains []string
	}{
		{
			name: "endpoint empty disables all other checks",
			mutate: func(c *Config) {
				// Start from a blank Config — defaults not applied — and keep
				// Endpoint empty. Validate must still return nil.
				*c = Config{}
			},
			wantErr: false,
		},
		{
			name: "valid happy path",
			mutate: func(*Config) {
				// no-op: use the valid baseline.
			},
			wantErr: false,
		},
		{
			name: "bad scheme",
			mutate: func(c *Config) {
				c.Endpoint = "ftp://example.com/"
			},
			wantErr:     true,
			errContains: []string{"notifications.endpoint must be http(s) URL"},
		},
		{
			name: "missing scheme",
			mutate: func(c *Config) {
				c.Endpoint = "example.com/hook"
			},
			wantErr:     true,
			errContains: []string{"notifications.endpoint must be http(s) URL"},
		},
		{
			name: "reserved Content-Type header",
			mutate: func(c *Config) {
				c.Headers.Set("Content-Type", configopaque.String("application/xml"))
			},
			wantErr:     true,
			errContains: []string{"Content-Type"},
		},
		{
			name: "reserved content-type header mixed case",
			mutate: func(c *Config) {
				c.Headers.Set("content-TYPE", configopaque.String("application/xml"))
			},
			wantErr:     true,
			errContains: []string{"Content-Type"},
		},
		{
			name: "reserved Content-Encoding header",
			mutate: func(c *Config) {
				c.Headers.Set("Content-Encoding", configopaque.String("gzip"))
			},
			wantErr:     true,
			errContains: []string{"Content-Encoding"},
		},
		{
			name: "compression configured",
			mutate: func(c *Config) {
				c.Compression = "gzip"
			},
			wantErr:     true,
			errContains: []string{"notifications.compression is not supported"},
		},
		{
			// "none" is the explicit uncompressed sentinel in configcompression.
			name: "compression none accepted",
			mutate: func(c *Config) {
				c.Compression = "none"
			},
			wantErr: false,
		},
		{
			name: "compression empty accepted",
			mutate: func(c *Config) {
				c.Compression = ""
			},
			wantErr: false,
		},
		{
			name: "endpoint missing host",
			mutate: func(c *Config) {
				c.Endpoint = "http://"
			},
			wantErr:     true,
			errContains: []string{"notifications.endpoint must be http(s) URL"},
		},
		{
			name: "endpoint no authority",
			mutate: func(c *Config) {
				c.Endpoint = "http:/hook"
			},
			wantErr:     true,
			errContains: []string{"notifications.endpoint must be http(s) URL"},
		},
		{
			name: "endpoint empty authority",
			mutate: func(c *Config) {
				c.Endpoint = "https:///hook"
			},
			wantErr:     true,
			errContains: []string{"notifications.endpoint must be http(s) URL"},
		},
		{
			name: "queue_size zero",
			mutate: func(c *Config) {
				c.QueueSize = 0
			},
			wantErr:     true,
			errContains: []string{"queue_size must be >= 1"},
		},
		{
			name: "workers zero",
			mutate: func(c *Config) {
				c.Workers = 0
			},
			wantErr:     true,
			errContains: []string{"workers must be >= 1"},
		},
		{
			name: "max_records_per_post zero",
			mutate: func(c *Config) {
				c.MaxRecordsPerPost = 0
			},
			wantErr:     true,
			errContains: []string{"max_records_per_post must be >= 1"},
		},
		{
			name: "max_attempts zero",
			mutate: func(c *Config) {
				c.MaxAttempts = 0
			},
			wantErr:     true,
			errContains: []string{"max_attempts must be >= 1"},
		},
		{
			name: "initial_backoff zero",
			mutate: func(c *Config) {
				c.InitialBackoff = 0
			},
			wantErr: true,
			// When InitialBackoff == 0 and MaxBackoff > 0, only the first check fires.
			// Likewise keep MaxBackoff >= InitialBackoff to avoid a spurious second error.
			errContains: []string{"initial_backoff must be > 0"},
		},
		{
			name: "max_backoff less than initial_backoff",
			mutate: func(c *Config) {
				c.InitialBackoff = 5 * time.Second
				c.MaxBackoff = 1 * time.Second
			},
			wantErr:     true,
			errContains: []string{"max_backoff must be >= initial_backoff"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := valid()
			tc.mutate(&c)
			err := c.Validate()
			if tc.wantErr {
				require.Error(t, err)
				for _, s := range tc.errContains {
					assert.ErrorContains(t, err, s)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateSkipsWhenEndpointEmpty asserts that otherwise-invalid
// notification settings do not surface errors when the feature is off. This
// is what allows createDefaultConfig() to embed NewDefaultConfig() without
// Validate failing on a dormant block that has the defaults populated.
func TestValidateSkipsWhenEndpointEmpty(t *testing.T) {
	t.Parallel()

	c := NewDefaultConfig()
	c.QueueSize = -1
	c.Workers = 0
	c.MaxAttempts = 0
	c.InitialBackoff = 0
	// Endpoint remains empty.

	assert.NoError(t, c.Validate())
}
