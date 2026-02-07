// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
)

// otelNamespaceUUID is the official OTel namespace UUID for deterministic UUID v5 generation,
// as recommended by the semantic conventions for service.instance.id.
// See: https://opentelemetry.io/docs/specs/semconv/registry/attributes/service/
var otelNamespaceUUID = uuid.MustParse("4d63009a-8d0f-11ee-aad7-4c796ed8e320")

// Config is the configuration of a scraper.
type Config interface {
	SetRootPath(rootPath string)
}

func NewEnvVarFactory(delegate scraper.Factory, envMap common.EnvMap) scraper.Factory {
	return scraper.NewFactory(delegate.Type(), func() component.Config {
		return delegate.CreateDefaultConfig()
	}, scraper.WithMetrics(func(ctx context.Context, settings scraper.Settings, config component.Config) (scraper.Metrics, error) {
		scrp, err := delegate.CreateMetrics(ctx, settings, config)
		if err != nil {
			return nil, err
		}
		return &envVarScraper{delegate: scrp, envMap: envMap}, nil
	}, delegate.MetricsStability()))
}

type envVarScraper struct {
	delegate scraper.Metrics
	envMap   common.EnvMap
}

func (evs *envVarScraper) Start(ctx context.Context, host component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Start(ctx, host)
}

func (evs *envVarScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.ScrapeMetrics(ctx)
}

func (evs *envVarScraper) Shutdown(ctx context.Context) error {
	ctx = context.WithValue(ctx, common.EnvKey, evs.envMap)
	return evs.delegate.Shutdown(ctx)
}

// NewResourceAttributeFactory creates a factory that wraps scrapers with resource attribute injection.
// It adds service.instance.id to all emitted metrics.
func NewResourceAttributeFactory(delegate scraper.Factory, hostname string) scraper.Factory {
	baseInstanceID := generateServiceInstanceID(hostname)

	return scraper.NewFactory(delegate.Type(), func() component.Config {
		return delegate.CreateDefaultConfig()
	}, scraper.WithMetrics(func(ctx context.Context, settings scraper.Settings, config component.Config) (scraper.Metrics, error) {
		scrp, err := delegate.CreateMetrics(ctx, settings, config)
		if err != nil {
			return nil, err
		}
		return &resourceAttributeScraper{
			delegate:       scrp,
			baseInstanceID: baseInstanceID,
			scraperType:    delegate.Type(),
		}, nil
	}, delegate.MetricsStability()))
}

// resourceAttributeScraper wraps a scraper.Metrics to inject service.instance.id
// resource attribute into all emitted metrics.
type resourceAttributeScraper struct {
	delegate       scraper.Metrics
	baseInstanceID string // UUID v5 from hostname
	scraperType    component.Type
}

func (ras *resourceAttributeScraper) Start(ctx context.Context, host component.Host) error {
	return ras.delegate.Start(ctx, host)
}

func (ras *resourceAttributeScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	metrics, err := ras.delegate.ScrapeMetrics(ctx)

	// Inject resource attributes into all ResourceMetrics, even on partial errors.
	// Scrapers frequently return partial scrape errors along with valid metrics.
	ras.injectResourceAttributes(metrics)

	return metrics, err
}

func (ras *resourceAttributeScraper) Shutdown(ctx context.Context) error {
	return ras.delegate.Shutdown(ctx)
}

func (ras *resourceAttributeScraper) injectResourceAttributes(metrics pmetric.Metrics) {
	// Iterate through all ResourceMetrics
	for i := range metrics.ResourceMetrics().Len() {
		rm := metrics.ResourceMetrics().At(i)
		attrs := rm.Resource().Attributes()

		// Add service.instance.id
		// For process scraper, generate unique ID per process using process.pid
		if ras.scraperType.String() == "process" {
			if pidVal, ok := attrs.Get("process.pid"); ok {
				pid := pidVal.Int()
				instanceID := generateProcessServiceInstanceID(ras.baseInstanceID, pid)
				attrs.PutStr("service.instance.id", instanceID)
			} else {
				// Fallback to base instance ID if no PID found
				attrs.PutStr("service.instance.id", ras.baseInstanceID)
			}
		} else {
			// Host-level scrapers use base instance ID
			attrs.PutStr("service.instance.id", ras.baseInstanceID)
		}
	}
}

// generateServiceInstanceID creates a deterministic UUID v5 from hostname using
// the OTel namespace UUID.
func generateServiceInstanceID(hostname string) string {
	return uuid.NewSHA1(otelNamespaceUUID, []byte(hostname)).String()
}

// generateProcessServiceInstanceID creates a deterministic UUID v5 that is unique
// per process by combining the base hostname UUID with the process PID.
func generateProcessServiceInstanceID(baseInstanceID string, pid int64) string {
	combined := fmt.Sprintf("%s:%d", baseInstanceID, pid)
	return uuid.NewSHA1(otelNamespaceUUID, []byte(combined)).String()
}
