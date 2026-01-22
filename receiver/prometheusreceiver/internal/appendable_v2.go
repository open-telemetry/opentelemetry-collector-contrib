// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// appendableV2 translates Prometheus scraping diffs into OpenTelemetry format
// using the new AppenderV2 interface from Prometheus.
//
// Key differences from V1 (appendable):
// - Single Append method instead of 7+ separate methods
// - Metadata passed directly in opts.Metadata (no context lookup needed)
// - Exemplars passed directly in opts.Exemplars
// - Start timestamp passed as st parameter
// - No state tracking flags needed (addingNativeHistogram, addingNHCB)
type appendableV2 struct {
	sink           consumer.Metrics
	trimSuffixes   bool
	externalLabels labels.Labels
	settings       receiver.Settings
	obsrecv        *receiverhelper.ObsReport
}

// NewAppendableV2 returns a storage.AppendableV2 instance that emits metrics to the sink.
func NewAppendableV2(
	sink consumer.Metrics,
	set receiver.Settings,
	externalLabels labels.Labels,
	trimSuffixes bool,
) (storage.AppendableV2, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return &appendableV2{
		sink:           sink,
		settings:       set,
		externalLabels: externalLabels,
		obsrecv:        obsrecv,
		trimSuffixes:   trimSuffixes,
	}, nil
}

func (o *appendableV2) AppenderV2(ctx context.Context) storage.AppenderV2 {
	return newTransactionV2(ctx, o.sink, o.externalLabels, o.settings, o.obsrecv, o.trimSuffixes)
}
