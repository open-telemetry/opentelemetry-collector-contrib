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

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const (
	// ReceiverKey used to identify receivers in metrics and traces.
	receiverKey = "receiver"
	nameSep     = "/"
)

type receiverState uint8

const (
	receiverStateStarting receiverState = iota
	receiverStateConnecting
	receiverStateConnected
	receiverStateIdle
	receiverStateTerminating
	receiverStateTerminated
)

var (
	failedReconnections            = stats.Int64("solacereceiver/failed_reconnections", "Number of failed broker reconnections", stats.UnitDimensionless)
	recoverableUnmarshallingErrors = stats.Int64("solacereceiver/recoverable_unmarshalling_errors", "Number of recoverable message unmarshalling errors", stats.UnitDimensionless)
	fatalUnmarshallingErrors       = stats.Int64("solacereceiver/fatal_unmarshalling_errors", "Number of fatal message unmarshalling errors", stats.UnitDimensionless)
	droppedSpanMessages            = stats.Int64("solacereceiver/dropped_span_messages", "Number of dropped span messages", stats.UnitDimensionless)
	receivedSpanMessages           = stats.Int64("solacereceiver/received_span_messages", "Number of received span messages", stats.UnitDimensionless)
	reportedSpans                  = stats.Int64("solacereceiver/reported_spans", "Number of reported spans", stats.UnitDimensionless)
	receiverStatus                 = stats.Int64("solacereceiver/receiver_status", "Indicates the status of the receiver", stats.UnitDimensionless)
	needUpgrade                    = stats.Int64("solacereceiver/need_upgrade", "Indicates with value 1 that receiver requires an upgrade and is not compatible with messages received from a broker", stats.UnitDimensionless)
)

var (
	viewFailedReconnections            = fromMeasure(failedReconnections, view.Count())
	viewRecoverableUnmarshallingErrors = fromMeasure(recoverableUnmarshallingErrors, view.Count())
	viewFatalUnmarshallingErrors       = fromMeasure(fatalUnmarshallingErrors, view.Count())
	viewDroppedSpanMessages            = fromMeasure(droppedSpanMessages, view.Count())
	viewReceivedSpanMessages           = fromMeasure(receivedSpanMessages, view.Count())
	viewReportedSpans                  = fromMeasure(reportedSpans, view.Sum())
	viewReceiverStatus                 = fromMeasure(receiverStatus, view.LastValue())
	viewNeedUpgrade                    = fromMeasure(needUpgrade, view.LastValue())
)

// receiver will register internal telemetry views
func init() {
	registerMetrics()
}

func registerMetrics() {
	if err := view.Register(
		viewFailedReconnections,
		viewRecoverableUnmarshallingErrors,
		viewFatalUnmarshallingErrors,
		viewDroppedSpanMessages,
		viewReceivedSpanMessages,
		viewReportedSpans,
		viewReceiverStatus,
		viewNeedUpgrade,
	); err != nil {
		panic(err)
	}
}

func fromMeasure(measure stats.Measure, agg *view.Aggregation) *view.View {
	return &view.View{
		Name:        buildReceiverCustomMetricName(measure.Name()),
		Description: measure.Description(),
		Measure:     measure,
		Aggregation: agg,
	}
}

func buildReceiverCustomMetricName(metric string) string {
	return receiverKey + nameSep + string(componentType) + nameSep + metric
}

// recordFailedReconnection increments the metric that records failed reconnection event.
func recordFailedReconnection() {
	stats.Record(context.Background(), failedReconnections.M(1))
}

// recordRecoverableUnmarshallingError increments the metric that records a recoverable error by trace message unmarshalling.
func recordRecoverableUnmarshallingError() {
	stats.Record(context.Background(), recoverableUnmarshallingErrors.M(1))
}

// recordFatalUnmarshallingError increments the metric that records a fatal arrow by trace message unmarshalling.
func recordFatalUnmarshallingError() {
	stats.Record(context.Background(), fatalUnmarshallingErrors.M(1))
}

// recordDroppedSpanMessages increments the metric that records a dropped span message
func recordDroppedSpanMessages() {
	stats.Record(context.Background(), droppedSpanMessages.M(1))
}

// recordReceivedSpanMessages increments the metric that records a received span message
func recordReceivedSpanMessages() {
	stats.Record(context.Background(), receivedSpanMessages.M(1))
}

// recordReportedSpans increments the metric that records the number of spans reported to the next consumer
func recordReportedSpans() {
	stats.Record(context.Background(), reportedSpans.M(1))
}

// recordReceiverStatus sets the metric that records the current state of the receiver to the given state
func recordReceiverStatus(status receiverState) {
	stats.Record(context.Background(), receiverStatus.M(int64(status)))
}

// RecordNeedRestart turns a need restart flag on
func recordNeedUpgrade() {
	stats.Record(context.Background(), needUpgrade.M(1))
}
