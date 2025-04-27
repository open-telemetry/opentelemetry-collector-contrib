// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestToPrometheusConfig(t *testing.T) {
	// prepare
	prFactory := prometheusreceiver.NewFactory()
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.Create(context.Background(), extensiontest.NewNopSettings(baFactory.Type()), baCfg)
	require.NoError(t, err)

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewIDWithName("bearertokenauth", "array01"): baExt,
		},
	}

	endpoint := "http://example.com"
	namespace := "purefa"
	tlsSettings := configtls.ClientConfig{
		InsecureSkipVerify: true,
		ServerName:         "TestThisServerName",
	}
	interval := 15 * time.Second
	cfgs := []ScraperConfig{
		{
			Address: "array01",
			Auth: configauth.Authentication{
				AuthenticatorID: component.MustNewIDWithName("bearertokenauth", "array01"),
			},
		},
	}

	scraper := NewScraper(context.Background(), "hosts", endpoint, namespace, tlsSettings, cfgs, interval, model.LabelSet{})

	// test
	scCfgs, err := scraper.ToPrometheusReceiverConfig(host, prFactory)

	// verify
	assert.NoError(t, err)
	assert.Len(t, scCfgs, 1)
	assert.NotNil(t, scCfgs[0].ScrapeProtocols)
	assert.Equal(t, "purefa", scCfgs[0].Params.Get("namespace"))
	assert.EqualValues(t, "the-token", scCfgs[0].HTTPClientConfig.BearerToken)
	assert.True(t, scCfgs[0].HTTPClientConfig.TLSConfig.InsecureSkipVerify)
	assert.Equal(t, "TestThisServerName", scCfgs[0].HTTPClientConfig.TLSConfig.ServerName)
	assert.Equal(t, "array01", scCfgs[0].Params.Get("endpoint"))
	assert.Equal(t, "/metrics/hosts", scCfgs[0].MetricsPath)
	assert.Equal(t, "purefa/hosts/array01", scCfgs[0].JobName)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeTimeout)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeInterval)
}
