// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type cfGardenObserver struct {
	*observer.EndpointsWatcher
}

var _ extension.Extension = (*cfGardenObserver)(nil)

func newObserver(params extension.Settings, config *Config) (extension.Extension, error) {
	g := &cfGardenObserver{}
	g.EndpointsWatcher = observer.NewEndpointsWatcher(g, time.Second, params.Logger)

	return g, nil
}

func (g *cfGardenObserver) Start(context.Context, component.Host) error {
	return nil
}

func (g *cfGardenObserver) Shutdown(context.Context) error {
	return nil
}

func (g *cfGardenObserver) ListEndpoints() []observer.Endpoint {
	// TODO: Implement the logic to list the endpoints.
	endpoints := make([]observer.Endpoint, 0)

	return endpoints
}
