// Copyright  The OpenTelemetry Authors
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

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
)

// The metricsConsumer implements the firehoseConsumer
// to use a metrics consumer and unmarshaler.
type metricsConsumer struct {
	// consumer passes the translated metrics on to the
	// next consumer.
	consumer consumer.Metrics
	// unmarshaler is the configured MetricsUnmarshaler
	// to use when processing the records.
	unmarshaler unmarshaler.MetricsUnmarshaler
}

var _ firehoseConsumer = (*metricsConsumer)(nil)

// newMetricsReceiver creates a new instance of the receiver
// with a metricsConsumer.
func newMetricsReceiver(
	config *Config,
	set component.ReceiverCreateSettings,
	unmarshalers map[string]unmarshaler.MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	configuredUnmarshaler := unmarshalers[config.RecordType]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedRecordType
	}

	mc := &metricsConsumer{
		consumer:    nextConsumer,
		unmarshaler: configuredUnmarshaler,
	}

	return &firehoseReceiver{
		instanceID: config.ID(),
		settings:   set,
		config:     config,
		consumer:   mc,
	}, nil
}

// Consume uses the configured unmarshaler to deserialize the records into a
// single pdata.Metrics. If there are common attributes available, then it will
// attach those to each of the pdata.Resources. It will send the final result
// to the next consumer.
func (mc *metricsConsumer) Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) (int, error) {
	md, err := mc.unmarshaler.Unmarshal(records)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if commonAttributes != nil {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for k, v := range commonAttributes {
				rm.Resource().Attributes().InsertString(k, v)
			}
		}
	}

	err = mc.consumer.ConsumeMetrics(ctx, md)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
