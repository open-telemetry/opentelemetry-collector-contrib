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

// appendable translates Prometheus scraping diffs into OpenTelemetry format.
// It implements storage.AppendableV2.
type appendable struct {
	sink           consumer.Metrics
	useMetadata    bool
	trimSuffixes   bool
	externalLabels labels.Labels

	settings receiver.Settings
	obsrecv  *receiverhelper.ObsReport
}

// NewAppendable returns an appendable instance that emits metrics to the sink.
func NewAppendable(
	sink consumer.Metrics,
	set receiver.Settings,
	useMetadata bool,
	externalLabels labels.Labels,
	trimSuffixes bool,
) (storage.AppendableV2, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{ReceiverID: set.ID, Transport: transport, ReceiverCreateSettings: set})
	if err != nil {
		return nil, err
	}

	return &appendable{
		sink:           sink,
		settings:       set,
		useMetadata:    useMetadata,
		externalLabels: externalLabels,
		obsrecv:        obsrecv,
		trimSuffixes:   trimSuffixes,
	}, nil
}

func (o *appendable) AppenderV2(ctx context.Context) storage.AppenderV2 {
	return newTransaction(ctx, o.sink, o.externalLabels, o.settings, o.obsrecv, o.trimSuffixes, o.useMetadata)
}
