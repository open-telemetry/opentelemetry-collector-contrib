// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

type systemdScraper struct {
	mb *metadata.MetricsBuilder
}

func (*systemdScraper) start(_ context.Context, _ component.Host) error {
	return errNonLinux
}

func (*systemdScraper) shutdown(_ context.Context) error {
	return errNonLinux
}

func (s *systemdScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	return s.mb.Emit(), errNonLinux
}

func newScraper(conf *Config, settings receiver.Settings) *systemdScraper {
	return &systemdScraper{mb: metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings)}
}
