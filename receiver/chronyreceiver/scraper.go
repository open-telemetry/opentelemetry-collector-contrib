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

package chronyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver"

import (
	"context"

	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

type chronyScraper struct {
	client chrony.Client
	mb     *metadata.MetricsBuilder
}

func newScraper(ctx context.Context, client chrony.Client, cfg *Config, set receiver.CreateSettings) *chronyScraper {
	return &chronyScraper{
		client: client,
		mb: metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, set,
			metadata.WithStartTime(pcommon.NewTimestampFromTime(clock.FromContext(ctx).Now())),
		),
	}
}

func (cs *chronyScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	data, err := cs.client.GetTrackingData(ctx)
	if err != nil {
		return pmetric.Metrics{}, err
	}

	now := pcommon.NewTimestampFromTime(clock.FromContext(ctx).Now())

	cs.mb.RecordNtpStratumDataPoint(now, int64(data.Stratum))
	cs.mb.RecordNtpTimeCorrectionDataPoint(
		now,
		data.CurrentCorrection,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	cs.mb.RecordNtpTimeLastOffsetDataPoint(
		now,
		data.LastOffset,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	cs.mb.RecordNtpTimeRmsOffsetDataPoint(
		now,
		data.RMSOffset,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	cs.mb.RecordNtpFrequencyOffsetDataPoint(
		now,
		data.FreqPPM,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	cs.mb.RecordNtpSkewDataPoint(now, data.SkewPPM)
	cs.mb.RecordNtpTimeRootDelayDataPoint(
		now,
		data.RootDelay,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)

	return cs.mb.Emit(), nil
}
