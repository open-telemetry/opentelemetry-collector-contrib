// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal"

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

func TestToPrometheusConfig(t *testing.T) {
	// prepare
	prFactory := prometheusreceiver.NewFactory()
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.Create(t.Context(), extensiontest.NewNopSettings(baFactory.Type()), baCfg)
	require.NoError(t, err)

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewIDWithName("bearertokenauth", "fb01"): baExt,
		},
	}

	endpoint := "http://example.com"
	interval := 5 * time.Second
	cfgs := []ScraperConfig{
		{
			Address: "fb01",
			Auth: configauth.Config{
				AuthenticatorID: component.MustNewIDWithName("bearertokenauth", "fb01"),
			},
		},
	}

	scraper := NewScraper(t.Context(), "hosts", endpoint, cfgs, interval, model.LabelSet{})

	// test
	scCfgs, err := scraper.ToPrometheusReceiverConfig(host, prFactory)

	// verify
	assert.NoError(t, err)
	assert.Len(t, scCfgs, 1)
	assert.EqualValues(t, "the-token", scCfgs[0].HTTPClientConfig.BearerToken)
	assert.Equal(t, "fb01", scCfgs[0].Params.Get("endpoint"))
	assert.Equal(t, "/metrics/hosts", scCfgs[0].MetricsPath)
	assert.Equal(t, "purefb/hosts/fb01", scCfgs[0].JobName)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeTimeout)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeInterval)
}

// TestToPrometheusConfigMarshalsToYAML guards against using a *discovery.StaticConfig
// pointer, which is unregistered and fails the YAML marshal the prometheus receiver
// runs on startup.
func TestToPrometheusConfigMarshalsToYAML(t *testing.T) {
	prFactory := prometheusreceiver.NewFactory()
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.Create(t.Context(), extensiontest.NewNopSettings(baFactory.Type()), baCfg)
	require.NoError(t, err)

	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewIDWithName("bearertokenauth", "fb01"): baExt,
		},
	}

	cfgs := []ScraperConfig{
		{
			Address: "fb01",
			Auth: configauth.Config{
				AuthenticatorID: component.MustNewIDWithName("bearertokenauth", "fb01"),
			},
		},
	}

	scraper := NewScraper(t.Context(), "hosts", "http://example.com", cfgs, 5*time.Second, model.LabelSet{})

	scCfgs, err := scraper.ToPrometheusReceiverConfig(host, prFactory)
	require.NoError(t, err)
	require.Len(t, scCfgs, 1)

	// Mirrors the marshaling the prometheus receiver performs on startup.
	_, err = yaml.Marshal(scCfgs[0])
	require.NoError(t, err)
}
