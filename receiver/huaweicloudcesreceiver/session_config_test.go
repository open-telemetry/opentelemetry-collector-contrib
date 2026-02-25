// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHTTPConfigNoVerifySSL(t *testing.T) {
	cfg, err := createHTTPConfig(huaweiSessionConfig{NoVerifySSL: true})
	require.NoError(t, err)
	assert.True(t, cfg.IgnoreSSLVerification)
}

func TestCreateHTTPConfigWithProxy(t *testing.T) {
	cfg, err := createHTTPConfig(huaweiSessionConfig{
		ProxyAddress:  "https://127.0.0.1:8888",
		ProxyUser:     "admin",
		ProxyPassword: "pass",
		AccessKey:     "123",
		SecretKey:     "secret",
	})
	require.NoError(t, err)
	assert.Equal(t, "https", cfg.HttpProxy.Schema)
	assert.Equal(t, "127.0.0.1", cfg.HttpProxy.Host)
	assert.Equal(t, 8888, cfg.HttpProxy.Port)
	assert.False(t, cfg.IgnoreSSLVerification)
}
