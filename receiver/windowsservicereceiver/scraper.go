// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

type windowsServiceScraper struct {
	cfg      *Config
	settings *receiver.Settings
}

func (wss *windowsServiceScraper) Start(ctx context.Context, _ component.Host) error {

	return nil
}

func (wss *windowsServiceScraper) Shutdown(_ context.Context) error {
	return nil
}
