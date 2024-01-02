// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
)

const (
	// receiverKey used to identify receivers in metrics and traces.
	receiverKey = "receiver"
	// metricPrefix used to prefix solace specific metrics
	metricPrefix = "solacereceiver"
	nameSep      = "/"
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

type flowControlState uint8

const (
	flowControlStateClear flowControlState = iota
	flowControlStateControlled
)

type opencensusMetrics struct {
	stats struct {
		failedReconnections            *stats.Int64Measure
		recoverableUnmarshallingErrors *stats.Int64Measure
		fatalUnmarshallingErrors       *stats.Int64Measure
		droppedSpanMessages            *stats.Int64Measure
		receivedSpanMessages           *stats.Int64Measure
		reportedSpans                  *stats.Int64Measure
		receiverStatus                 *stats.Int64Measure
		needUpgrade                    *stats.Int64Measure
		flowControlStatus              *stats.Int64Measure
		flowControlRecentRetries       *stats.Int64Measure
		flowControlTotal               *stats.Int64Measure
		flowControlSingleSuccess       *stats.Int64Measure
		droppedEgressSpans             *stats.Int64Measure
	}
	views struct {
		failedReconnections            *view.View
		recoverableUnmarshallingErrors *view.View
		fatalUnmarshallingErrors       *view.View
		droppedSpanMessages            *view.View
		receivedSpanMessages           *view.View
		reportedSpans                  *view.View
		receiverStatus                 *view.View
		needUpgrade                    *view.View
		flowControlStatus              *view.View
		flowControlRecentRetries       *view.View
		flowControlTotal               *view.View
		flowControlSingleSuccess       *view.View
		droppedEgressSpans             *view.View
	}
}

// receiver will register internal telemetry views
func newOpenCensusMetrics(instanceName string) (*opencensusMetrics, error) {
	m := &opencensusMetrics{}
	prefix := metricPrefix + nameSep
	if instanceName != "" {
		prefix += instanceName + nameSep
	}

	m.stats.failedReconnections = stats.Int64(prefix+"failed_reconnections", "Number of failed broker reconnections", stats.UnitDimensionless)
	m.stats.recoverableUnmarshallingErrors = stats.Int64(prefix+"recoverable_unmarshalling_errors", "Number of recoverable message unmarshalling errors", stats.UnitDimensionless)
	m.stats.fatalUnmarshallingErrors = stats.Int64(prefix+"fatal_unmarshalling_errors", "Number of fatal message unmarshalling errors", stats.UnitDimensionless)
	m.stats.droppedSpanMessages = stats.Int64(prefix+"dropped_span_messages", "Number of dropped span messages", stats.UnitDimensionless)
	m.stats.receivedSpanMessages = stats.Int64(prefix+"received_span_messages", "Number of received span messages", stats.UnitDimensionless)
	m.stats.reportedSpans = stats.Int64(prefix+"reported_spans", "Number of reported spans", stats.UnitDimensionless)
	m.stats.receiverStatus = stats.Int64(prefix+"receiver_status", "Indicates the status of the receiver as an enum. 0 = starting, 1 = connecting, 2 = connected, 3 = disabled (often paired with needs_upgrade), 4 = terminating, 5 = terminated", stats.UnitDimensionless)
	m.stats.needUpgrade = stats.Int64(prefix+"need_upgrade", "Indicates with value 1 that receiver requires an upgrade and is not compatible with messages received from a broker", stats.UnitDimensionless)

	m.stats.flowControlStatus = stats.Int64(prefix+"receiver_flow_control_status", "Indicates the flow control status of the receiver. 0 = not flow controlled, 1 = currently flow controlled", stats.UnitDimensionless)
	m.stats.flowControlRecentRetries = stats.Int64(prefix+"receiver_flow_control_recent_retries", "Most recent/current retry count when flow controlled", stats.UnitDimensionless)
	m.stats.flowControlTotal = stats.Int64(prefix+"receiver_flow_control_total", "Number of times the receiver instance became flow controlled", stats.UnitDimensionless)
	m.stats.flowControlSingleSuccess = stats.Int64(prefix+"receiver_flow_control_with_single_successful_retry", "Number of times the receiver instance became flow controlled and resolved situations after the first retry", stats.UnitDimensionless)

	m.stats.droppedEgressSpans = stats.Int64(prefix+"dropped_egress_spans", "Number of dropped egress spans", stats.UnitDimensionless)

	m.views.failedReconnections = fromMeasure(m.stats.failedReconnections, view.Count())
	m.views.recoverableUnmarshallingErrors = fromMeasure(m.stats.recoverableUnmarshallingErrors, view.Count())
	m.views.fatalUnmarshallingErrors = fromMeasure(m.stats.fatalUnmarshallingErrors, view.Count())
	m.views.droppedSpanMessages = fromMeasure(m.stats.droppedSpanMessages, view.Count())
	m.views.receivedSpanMessages = fromMeasure(m.stats.receivedSpanMessages, view.Count())
	m.views.reportedSpans = fromMeasure(m.stats.reportedSpans, view.Sum())
	m.views.receiverStatus = fromMeasure(m.stats.receiverStatus, view.LastValue())
	m.views.needUpgrade = fromMeasure(m.stats.needUpgrade, view.LastValue())

	m.views.flowControlStatus = fromMeasure(m.stats.flowControlStatus, view.LastValue())
	m.views.flowControlRecentRetries = fromMeasure(m.stats.flowControlRecentRetries, view.LastValue())
	m.views.flowControlTotal = fromMeasure(m.stats.flowControlTotal, view.Count())
	m.views.flowControlSingleSuccess = fromMeasure(m.stats.flowControlSingleSuccess, view.Count())

	m.views.droppedEgressSpans = fromMeasure(m.stats.droppedEgressSpans, view.Count())

	err := view.Register(
		m.views.failedReconnections,
		m.views.recoverableUnmarshallingErrors,
		m.views.fatalUnmarshallingErrors,
		m.views.droppedSpanMessages,
		m.views.receivedSpanMessages,
		m.views.reportedSpans,
		m.views.receiverStatus,
		m.views.needUpgrade,
		m.views.flowControlStatus,
		m.views.flowControlRecentRetries,
		m.views.flowControlTotal,
		m.views.flowControlSingleSuccess,
		m.views.droppedEgressSpans,
	)
	if err != nil {
		return nil, err
	}
	return m, nil
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
	return receiverKey + nameSep + string(metadata.Type) + nameSep + metric
}

// recordFailedReconnection increments the metric that records failed reconnection event.
func (m *opencensusMetrics) recordFailedReconnection() {
	stats.Record(context.Background(), m.stats.failedReconnections.M(1))
}

// recordRecoverableUnmarshallingError increments the metric that records a recoverable error by trace message unmarshalling.
func (m *opencensusMetrics) recordRecoverableUnmarshallingError() {
	stats.Record(context.Background(), m.stats.recoverableUnmarshallingErrors.M(1))
}

// recordFatalUnmarshallingError increments the metric that records a fatal arrow by trace message unmarshalling.
func (m *opencensusMetrics) recordFatalUnmarshallingError() {
	stats.Record(context.Background(), m.stats.fatalUnmarshallingErrors.M(1))
}

// recordDroppedSpanMessages increments the metric that records a dropped span message
func (m *opencensusMetrics) recordDroppedSpanMessages() {
	stats.Record(context.Background(), m.stats.droppedSpanMessages.M(1))
}

// recordReceivedSpanMessages increments the metric that records a received span message
func (m *opencensusMetrics) recordReceivedSpanMessages() {
	stats.Record(context.Background(), m.stats.receivedSpanMessages.M(1))
}

// recordReportedSpans increments the metric that records the number of spans reported to the next consumer
func (m *opencensusMetrics) recordReportedSpans(amount int64) {
	stats.Record(context.Background(), m.stats.reportedSpans.M(amount))
}

// recordReceiverStatus sets the metric that records the current state of the receiver to the given state
func (m *opencensusMetrics) recordReceiverStatus(status receiverState) {
	stats.Record(context.Background(), m.stats.receiverStatus.M(int64(status)))
}

// RecordNeedRestart turns a need restart flag on
func (m *opencensusMetrics) recordNeedUpgrade() {
	stats.Record(context.Background(), m.stats.needUpgrade.M(1))
}

func (m *opencensusMetrics) recordFlowControlStatus(status flowControlState) {
	stats.Record(context.Background(), m.stats.flowControlStatus.M(int64(status)))
}

func (m *opencensusMetrics) recordFlowControlRecentRetries(retries int64) {
	stats.Record(context.Background(), m.stats.flowControlRecentRetries.M(retries))
}

func (m *opencensusMetrics) recordFlowControlTotal() {
	stats.Record(context.Background(), m.stats.flowControlTotal.M(1))
}

func (m *opencensusMetrics) recordFlowControlSingleSuccess() {
	stats.Record(context.Background(), m.stats.flowControlSingleSuccess.M(1))
}

func (m *opencensusMetrics) recordDroppedEgressSpan() {
	stats.Record(context.Background(), m.stats.droppedEgressSpans.M(1))
}
