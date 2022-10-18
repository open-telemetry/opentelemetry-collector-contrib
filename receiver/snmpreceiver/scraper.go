// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// snmpScraper handles scraping of SNMP metrics
type snmpScraper struct {
	logger   *zap.Logger
	cfg      *Config
	settings component.ReceiverCreateSettings
}

// newScraper creates an initialized snmpScraper
func newScraper(logger *zap.Logger, cfg *Config, settings component.ReceiverCreateSettings) *snmpScraper {
	return &snmpScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings,
	}
}

// start gets the client ready
func (s *snmpScraper) start(_ context.Context, _ component.Host) (err error) {
	return nil
}

// scrape collects and creates OTEL metrics from a SNMP environment
func (s *snmpScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()

	return md, nil
}
