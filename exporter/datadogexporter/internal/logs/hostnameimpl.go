// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"

import (
	"context"

	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
)

type service struct {
	name string
}

var _ hostnameinterface.Component = (*service)(nil)

// Get returns the hostname.
func (hs *service) Get(context.Context) (string, error) {
	return hs.name, nil
}

// GetSafe returns the hostname, or 'unknown host' if anything goes wrong.
func (hs *service) GetSafe(ctx context.Context) string {
	name, err := hs.Get(ctx)
	if err != nil {
		return "unknown host"
	}
	return name
}

// GetWithProvider returns the hostname for the Agent and the provider that was use to retrieve it.
func (hs *service) GetWithProvider(context.Context) (hostnameinterface.Data, error) {
	return hostnameinterface.Data{
		Hostname: hs.name,
		Provider: "",
	}, nil
}

// NewHostnameService creates a new instance of the component hostname
func NewHostnameService(ctx context.Context, provider source.Provider) (hostnameinterface.Component, error) {
	src, err := provider.Source(ctx)
	if err != nil {
		return nil, err
	}

	hostname := ""
	if src.Kind == source.HostnameKind {
		hostname = src.Identifier
	}
	return &service{
		name: hostname,
	}, nil
}
