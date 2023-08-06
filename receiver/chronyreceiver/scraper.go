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
	rmb := cs.mb.ResourceMetricsBuilder(pcommon.NewResource())

	rmb.RecordNtpStratumDataPoint(now, int64(data.Stratum))
	rmb.RecordNtpTimeCorrectionDataPoint(
		now,
		data.CurrentCorrection,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	rmb.RecordNtpTimeLastOffsetDataPoint(
		now,
		data.LastOffset,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	rmb.RecordNtpTimeRmsOffsetDataPoint(
		now,
		data.RMSOffset,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	rmb.RecordNtpFrequencyOffsetDataPoint(
		now,
		data.FreqPPM,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)
	rmb.RecordNtpSkewDataPoint(now, data.SkewPPM)
	rmb.RecordNtpTimeRootDelayDataPoint(
		now,
		data.RootDelay,
		metadata.AttributeLeapStatus(data.LeapStatus+1),
	)

	return cs.mb.Emit(), nil
}
