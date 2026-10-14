// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// TestGetPrometheusConfigInlineTLS proves that inline PEM TLS material and the
// server name are forwarded to the underlying prometheus receiver. Both are
// valid confighttp.ClientConfig TLS options; dropping them silently turns a
// TLS scrape into a misconfigured one with no error.
func TestGetPrometheusConfigInlineTLS(t *testing.T) {
	caPem, err := os.ReadFile("./testdata/test_cert.pem")
	require.NoError(t, err)

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAPem: configopaque.String(caPem),
		},
		ServerName: "example.com",
	}

	cfg := &Config{
		ClientConfig:       clientConfig,
		CollectionInterval: 10 * time.Second,
		MetricsPath:        "/metrics",
	}

	got, err := getPrometheusConfig(cfg)
	require.NoError(t, err)

	tlsCfg := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig.TLSConfig
	assert.Equal(t, string(caPem), tlsCfg.CA, "inline CA PEM (ca_pem) must be forwarded to the prometheus scrape config")
	assert.Equal(t, "example.com", tlsCfg.ServerName, "server_name must be forwarded to the prometheus scrape config")
}
