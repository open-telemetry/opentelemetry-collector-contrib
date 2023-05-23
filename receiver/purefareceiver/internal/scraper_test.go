// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

type MockHost struct {
	Extensions map[component.ID]component.Component
}

func (m *MockHost) ReportFatalError(err error) {}

func (m *MockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return nil
}

func (m *MockHost) GetExtensions() map[component.ID]component.Component {
	return m.Extensions
}

func (m *MockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
    return nil 
}
func TestToPrometheusConfig(t *testing.T) {

	// Creates a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve mock response for the test
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Mock response"))
		if err != nil {
			log.Println("Error on the response:", err)

			return
		}
	}))
	defer mockServer.Close()

	// prepare
	prFactory := prometheusreceiver.NewFactory()
	baFactory := bearertokenauthextension.NewFactory()

	baCfg := baFactory.CreateDefaultConfig().(*bearertokenauthextension.Config)
	baCfg.BearerToken = "the-token"

	baExt, err := baFactory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), baCfg)
	require.NoError(t, err)

	host := &MockHost{	
		Extensions: map[component.ID]component.Component{
			component.NewIDWithName("bearertokenauth", "array01"): baExt,
		},
	}

	endpoint := mockServer.URL
	interval := 15 * time.Second
	cfgs := []ScraperConfig{
		{
			Address: "array01",
			Auth: configauth.Authentication{
				AuthenticatorID: component.NewIDWithName("bearertokenauth", "array01"),
			},
		},
	}

	scraper := NewScraper(context.Background(), "hosts", endpoint, cfgs, interval, model.LabelSet{})
	
	// test
	scCfgs, err := scraper.ToPrometheusReceiverConfig(host, prFactory)

	// verify
	assert.NoError(t, err)
	assert.Len(t, scCfgs, 1)
	assert.EqualValues(t, "the-token", scCfgs[0].HTTPClientConfig.BearerToken)
	assert.Equal(t, "array01", scCfgs[0].Params.Get("endpoint"))
	assert.Equal(t, "/metrics/hosts", scCfgs[0].MetricsPath)
	assert.Equal(t, "purefa/hosts/array01", scCfgs[0].JobName)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeTimeout)
	assert.EqualValues(t, interval, scCfgs[0].ScrapeInterval)
}
