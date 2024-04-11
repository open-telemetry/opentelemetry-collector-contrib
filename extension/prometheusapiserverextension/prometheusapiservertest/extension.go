// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiservertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension/prometheusapiservertest"

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
)

var testPrometheusAPIServerType component.Type = component.MustNewType("prometheus_api_server_extension")

type TestPrometheusAPIServer struct {
	component.StartFunc
	component.ShutdownFunc
	ID         component.ID
	func (e *TestPrometheusAPIServer) RegisterPrometheusReceiverComponents(*config.Config, *scrape.Manager, prometheus.Registerer) error
}

func newPrometheusServerAPIExtensionID(name string) component.ID {
	return component.NewIDWithName(testPrometheusAPIServerType, name)
}

// NewFileBackedStorageExtension creates a TestStorage extension
func NewTestPrometheusAPIExtension(name string) TestPrometheusAPIServer {
	return TestPrometheusAPIServer{
		ID: newPrometheusServerAPIExtensionID(name),
	}
}