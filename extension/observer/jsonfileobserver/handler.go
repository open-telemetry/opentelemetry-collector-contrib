// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver"

import (
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/endpointswatcher"
)

var _ endpointswatcher.EndpointsLister = (*handler)(nil)

// handler provides a mechanism to list current endpoints from the JSON file.
type handler struct {
	idNamespace string
	endpoints   *sync.Map
	logger      *zap.Logger
}

// ListEndpoints returns all currently discovered endpoints from the JSON file.
func (h *handler) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	h.endpoints.Range(func(_, value any) bool {
		endpoints = append(endpoints, value.(observer.Endpoint))
		return true
	})
	return endpoints
}
