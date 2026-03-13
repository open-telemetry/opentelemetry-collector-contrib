// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/scraper/xscraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
)

var _ xscraper.Profiles = &HTTPClientScraper{}

type HTTPClientScraper struct {
	ClientConfig confighttp.ClientConfig
	Settings     component.TelemetrySettings
	client       *http.Client
}

func (hcs *HTTPClientScraper) Start(ctx context.Context, host component.Host) error {
	httpClient, err := hcs.ClientConfig.ToClient(ctx, host.GetExtensions(), hcs.Settings)
	hcs.client = httpClient
	return err
}

func (hcs *HTTPClientScraper) Shutdown(_ context.Context) error {
	if hcs.client != nil {
		hcs.client.CloseIdleConnections()
		hcs.client = nil
	}
	return nil
}

func (hcs *HTTPClientScraper) ScrapeProfiles(_ context.Context) (pprofile.Profiles, error) {
	resp, err := hcs.client.Get(hcs.ClientConfig.Endpoint)
	if err != nil {
		hcs.Settings.Logger.Error("error requesting profile", zap.Error(err), zap.String("endpoint", hcs.ClientConfig.Endpoint))
		return pprofile.Profiles{}, err
	}

	pprofProfile, err := profile.Parse(resp.Body)
	if err != nil {
		return pprofile.Profiles{}, fmt.Errorf("failed to parse pprof data: %w", err)
	}

	profiles, err := pprof.ConvertPprofToProfiles(pprofProfile)
	if err != nil {
		return pprofile.Profiles{}, fmt.Errorf("failed to convert pprof to profiles: %w", err)
	}
	return *profiles, err
}
