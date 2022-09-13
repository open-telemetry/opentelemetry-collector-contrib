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
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type snmpScraper struct {
	client   client
	logger   *zap.Logger
	cfg      *Config
	settings component.TelemetrySettings
}

// newScraper creates an initialized snmpScraper
func newScraper(logger *zap.Logger, cfg *Config, settings component.ReceiverCreateSettings) *snmpScraper {
	return &snmpScraper{
		logger:   logger,
		cfg:      cfg,
		settings: settings.TelemetrySettings,
	}
}

func (s *snmpScraper) start(ctx context.Context, host component.Host) (err error) {
	s.client, err = newClient(s.cfg, host, s.settings, s.logger)
	if err != nil {
		return err
	}
	err = s.client.Connect()
	if err != nil {
		return err
	}

	return
}

func (s *snmpScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	metricSlice := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	now := pcommon.NewTimestampFromTime(time.Now())

	if err := s.scrapeScalarMetrics(now, &metricSlice); err != nil {
		return md, err
	}

	return md, nil
}

func (s *snmpScraper) scrapeScalarMetrics(now pcommon.Timestamp, metricSlice *pmetric.MetricSlice) error {
	scalarMetricNamesByOID := map[string]string{}
	scalarMetricOIDs := []string{}
	for name, metricCfg := range s.cfg.Metrics {
		if len(metricCfg.ScalarOIDs) > 0 {
			for i, oid := range metricCfg.ScalarOIDs {
				if !strings.HasPrefix(oid.OID, ".") {
					oid.OID = "." + oid.OID
					s.cfg.Metrics[name].ScalarOIDs[i].OID = oid.OID
				}
				scalarMetricOIDs = append(scalarMetricOIDs, oid.OID)
				scalarMetricNamesByOID[oid.OID] = name
			}
		}
	}

	snmpData, err := s.client.GetData(scalarMetricOIDs)
	if err != nil {
		return err
	}
	for _, data := range snmpData {
		metricName := scalarMetricNamesByOID[data.Name]
		metricCfg := s.cfg.Metrics[metricName]
		var metricAttributes []Attribute

		for _, scalarOID := range metricCfg.ScalarOIDs {
			if scalarOID.OID == data.Name {
				metricAttributes = scalarOID.Attributes
			}
		}

		var dps pmetric.NumberDataPointSlice
		builtMetric := metricSlice.AppendEmpty()

		builtMetric.SetName(metricName)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		if (metricCfg.Sum != SumMetric{}) {
			builtMetric.SetEmptySum()
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			}
			dps = builtMetric.Sum().DataPoints()
		} else {
			builtMetric.SetEmptyGauge()
			dps = builtMetric.Gauge().DataPoints()
		}

		dp := dps.AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntVal(gosnmp.ToBigInt(data.Value).Int64())

		for _, attribute := range metricAttributes {
			attributeCfg := s.cfg.Attributes[attribute.Name]
			attributeKey := attribute.Name
			if attributeCfg.Value != "" {
				attributeKey = attributeCfg.Value
			}
			dp.Attributes().PutString(attributeKey, attribute.Value)
		}
	}

	return nil
}
