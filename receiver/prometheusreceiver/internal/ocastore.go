// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

var idSeq int64

// OcaStore translates Prometheus scraping diffs into OpenCensus format.
type OcaStore struct {
	ctx context.Context

	sink                 consumer.Metrics
	jobsMap              *JobsMapPdata
	useStartTimeMetric   bool
	startTimeMetricRegex string
	receiverID           config.ComponentID
	externalLabels       labels.Labels

	settings component.ReceiverCreateSettings
}

// NewOcaStore returns an ocaStore instance, which can be acted as prometheus' scrape.Appendable
func NewOcaStore(
	ctx context.Context,
	sink consumer.Metrics,
	set component.ReceiverCreateSettings,
	gcInterval time.Duration,
	useStartTimeMetric bool,
	startTimeMetricRegex string,
	receiverID config.ComponentID,
	externalLabels labels.Labels) *OcaStore {
	var jobsMap *JobsMapPdata
	if !useStartTimeMetric {
		jobsMap = NewJobsMapPdata(gcInterval)
	}
	return &OcaStore{
		ctx:                  ctx,
		sink:                 sink,
		settings:             set,
		jobsMap:              jobsMap,
		useStartTimeMetric:   useStartTimeMetric,
		startTimeMetricRegex: startTimeMetricRegex,
		receiverID:           receiverID,
		externalLabels:       externalLabels,
	}
}

func (o *OcaStore) Appender(ctx context.Context) storage.Appender {
	return newTransactionPdata(
		ctx,
		&txConfig{
			jobsMap:              o.jobsMap,
			useStartTimeMetric:   o.useStartTimeMetric,
			startTimeMetricRegex: o.startTimeMetricRegex,
			receiverID:           o.receiverID,
			sink:                 o.sink,
			externalLabels:       o.externalLabels,
			settings:             o.settings,
		},
	)

}

// Close OcaStore as well as the internal metadataService.
func (o *OcaStore) Close() {
}
