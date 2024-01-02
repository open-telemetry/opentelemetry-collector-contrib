// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"regexp"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// appendable translates Prometheus scraping diffs into OpenTelemetry format.
type appendable struct {
	sink                 consumer.Metrics
	metricAdjuster       MetricsAdjuster
	useStartTimeMetric   bool
	trimSuffixes         bool
	startTimeMetricRegex *regexp.Regexp
	externalLabels       labels.Labels

	settings receiver.CreateSettings
	obsrecv  *receiverhelper.ObsReport
}

// NewAppendable returns a storage.Appendable instance that emits metrics to the sink.
func NewAppendable(
	sink consumer.Metrics,
	set receiver.CreateSettings,
	gcInterval time.Duration,
	useStartTimeMetric bool,
	startTimeMetricRegex *regexp.Regexp,
	useCreatedMetric bool,
	externalLabels labels.Labels,
	trimSuffixes bool) (storage.Appendable, error) {
	var metricAdjuster MetricsAdjuster
	if !useStartTimeMetric {
		metricAdjuster = NewInitialPointAdjuster(set.Logger, gcInterval, useCreatedMetric)
	} else {
		metricAdjuster = NewStartTimeMetricAdjuster(set.Logger, startTimeMetricRegex)
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverID: set.ID, Transport: transport, ReceiverCreateSettings: set})
	if err != nil {
		return nil, err
	}

	return &appendable{
		sink:                 sink,
		settings:             set,
		metricAdjuster:       metricAdjuster,
		useStartTimeMetric:   useStartTimeMetric,
		startTimeMetricRegex: startTimeMetricRegex,
		externalLabels:       externalLabels,
		obsrecv:              obsrecv,
		trimSuffixes:         trimSuffixes,
	}, nil
}

func (o *appendable) Appender(ctx context.Context) storage.Appender {
	return newTransaction(ctx, o.metricAdjuster, o.sink, o.externalLabels, o.settings, o.obsrecv, o.trimSuffixes)
}
