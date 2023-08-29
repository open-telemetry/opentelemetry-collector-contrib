// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	now := pcommon.NewTimestampFromTime(clock.Now(ctx))

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
