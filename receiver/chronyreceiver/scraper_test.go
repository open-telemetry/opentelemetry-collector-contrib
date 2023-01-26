// Copyright The OpenTelemetry Authors
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

package chronyreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

type mockClient struct {
	mock.Mock
	chrony.Client
}

func (mc *mockClient) GetTrackingData(_ context.Context) (*chrony.Tracking, error) {
	args := mc.Called()
	return args.Get(0).(*chrony.Tracking), args.Error(1)
}

func TestChronyScraper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario     string
		conf         *Config
		mockTracking *chrony.Tracking
		mockErr      error
		err          error
		metrics      pmetric.Metrics
	}{
		{
			scenario: "Successfully read default tracking information",
			conf: &Config{
				Metrics: metadata.DefaultMetricsBuilderConfig(),
			},
			mockTracking: &chrony.Tracking{
				SkewPPM:           1000.300,
				CurrentCorrection: 0.00043,
				LastOffset:        0.00034,
				LeapStatus:        0,
			},
			mockErr: nil,
			err:     nil,
			metrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rMetrics := metrics.ResourceMetrics().AppendEmpty()

				metric := rMetrics.ScopeMetrics().AppendEmpty()
				metric.Scope().SetName("otelcol/chrony receiver")
				metric.Scope().SetVersion("latest")

				m := metric.Metrics().AppendEmpty()
				m.SetName("ntp.skew")
				m.SetUnit("ppm")
				m.SetDescription("This is the estimated error bound on the frequency.")
				g := m.SetEmptyGauge().DataPoints().AppendEmpty()
				g.SetDoubleValue(1000.300)
				g.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
				g.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))

				m = metric.Metrics().AppendEmpty()
				m.SetName("ntp.time.correction")
				m.SetUnit("seconds")
				m.SetDescription("The number of seconds difference between the system's clock and the reference clock")
				g = m.SetEmptyGauge().DataPoints().AppendEmpty()
				g.Attributes().PutStr("leap.status", "normal")
				g.SetDoubleValue(0.00043)
				g.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
				g.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))

				m = metric.Metrics().AppendEmpty()
				m.SetName("ntp.time.last_offset")
				m.SetUnit("seconds")
				m.SetDescription("The estimated local offset on the last clock update")
				g = m.SetEmptyGauge().DataPoints().AppendEmpty()
				g.Attributes().PutStr("leap.status", "normal")
				g.SetDoubleValue(0.00034)
				g.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
				g.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(100, 0)))
				return metrics
			}(),
		},
		{
			scenario: "client failed to connect to chronyd",
			conf: &Config{
				Metrics: metadata.DefaultMetricsBuilderConfig(),
			},
			mockTracking: nil,
			mockErr:      errInvalidValue,
			metrics:      pmetric.Metrics{},
			err:          errInvalidValue,
		},
	}

	// Clock allows for us to pin the time to
	// simplify checking the metrics
	clck := clock.NewMock(time.Unix(100, 0))

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			chronym := &mockClient{}

			chronym.On("GetTrackingData").Return(tc.mockTracking, tc.mockErr)

			ctx := clock.Context(context.Background(), clck)
			scraper := newScraper(ctx, chronym, tc.conf, receivertest.NewNopCreateSettings())

			metrics, err := scraper.scrape(ctx)

			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
			assert.EqualValues(t, tc.metrics, metrics, "Must match the expected metrics")
			chronym.AssertExpectations(t)
		})
	}
}
