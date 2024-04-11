// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiservertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension/prometheusapiservertest"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
)

type PrometheusAPIServerHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func NewPrometheusAPIServerHost() *PrometheusAPIServerHost {
	return &PrometheusAPIServerHost{
		Host:       componenttest.NewNopHost(),
		extensions: make(map[component.ID]component.Component),
	}
}

func (h *PrometheusAPIServerHost) WithExtension(id component.ID, ext extension.Extension) *PrometheusAPIServerHost {
	h.extensions[id] = ext
	return h
}

func (h *PrometheusAPIServerHost) WithPrometheusServerAPIExtension(name string) *PrometheusAPIServerHost {
	ext := NewTestPrometheusAPIExtension(name)
	h.extensions[ext.ID] = ext
	return h
}

func (h *PrometheusAPIServerHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
