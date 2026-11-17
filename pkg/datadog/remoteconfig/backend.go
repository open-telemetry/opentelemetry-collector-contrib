// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig"

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/config/remote/api"
	pbgo "github.com/DataDog/datadog-agent/pkg/proto/pbgo/core"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

type ownedTransport struct {
	http.RoundTripper
	closeIdle func()
}

func (t ownedTransport) CloseIdleConnections() { t.closeIdle() }

func (e *remoteConfigExtension) newAPIClient(ctx context.Context, host component.Host, cfg model.Reader, backendURL *url.URL) (*api.HTTPClient, func(), error) {
	// Keep the pool's cleanup hook before adding OTel instrumentation, whose
	// transport does not expose CloseIdleConnections.
	settings := e.settings.TelemetrySettings
	settings.TracerProvider, settings.MeterProvider = nil, nil
	backend, err := e.config.Backend.ToClient(ctx, host.GetExtensions(), settings)
	if err != nil {
		return nil, nil, err
	}
	closer, ok := backend.Transport.(interface{ CloseIdleConnections() })
	if !ok {
		return nil, nil, errors.New("remote_config backend transport does not support connection cleanup")
	}
	backend.Transport = ownedTransport{
		RoundTripper: otelhttp.NewTransport(backend.Transport,
			otelhttp.WithTracerProvider(e.settings.TracerProvider),
			otelhttp.WithMeterProvider(e.settings.MeterProvider),
			otelhttp.WithPropagators(otel.GetTextMapPropagator())),
		closeIdle: closer.CloseIdleConnections,
	}
	backend.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client, err := api.NewHTTPClient(api.Auth{APIKey: string(e.config.API.Key)}, cfg, backendURL, api.WithHTTPClient(backend))
	if err != nil {
		backend.CloseIdleConnections()
		return nil, nil, err
	}
	return client, backend.CloseIdleConnections, nil
}

type boundedAPI struct {
	api.API
	lifetime context.Context
	timeout  time.Duration
}

func (b boundedAPI) requestContext(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(parent, b.timeout)
	stop := context.AfterFunc(b.lifetime, cancel)
	return ctx, func() { stop(); cancel() }
}

func (b boundedAPI) Fetch(ctx context.Context, r *pbgo.LatestConfigsRequest) (*pbgo.LatestConfigsResponse, error) {
	ctx, cancel := b.requestContext(ctx)
	defer cancel()
	return b.API.Fetch(ctx, r)
}

func (b boundedAPI) FetchOrgData(ctx context.Context) (*pbgo.OrgDataResponse, error) {
	ctx, cancel := b.requestContext(ctx)
	defer cancel()
	return b.API.FetchOrgData(ctx)
}

func (b boundedAPI) FetchOrgStatus(ctx context.Context) (*pbgo.OrgStatusResponse, error) {
	ctx, cancel := b.requestContext(ctx)
	defer cancel()
	return b.API.FetchOrgStatus(ctx)
}
