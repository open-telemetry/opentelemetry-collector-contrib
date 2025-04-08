// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netstats // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"

import (
	"context"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
)

const (
	// ExporterKey is an attribute name that identifies an
	// exporter component that produces internal metrics, logs,
	// and traces.
	ExporterKey = "exporter"

	// ReceiverKey is an attribute name that identifies an
	// receiver component that produces internal metrics, logs,
	// and traces.
	ReceiverKey = "receiver"

	// SentBytes is used to track bytes sent by exporters and receivers.
	SentBytes = "sent"

	// SentWireBytes is used to track bytes sent on the wire
	// (includes compression) by exporters and receivers.
	SentWireBytes = "sent_wire"

	// RecvBytes is used to track bytes received by exporters and receivers.
	RecvBytes = "recv"

	// RecvWireBytes is used to track bytes received on the wire
	// (includes compression) by exporters and receivers.
	RecvWireBytes = "recv_wire"

	// CompSize is used for compressed size histogram metrics.
	CompSize = "compressed_size"

	scopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

// NetworkReporter is a helper to add network-level observability to
// an exporter or receiver.
type NetworkReporter struct {
	isExporter    bool
	staticAttr    attribute.KeyValue
	sentBytes     metric.Int64Counter
	sentWireBytes metric.Int64Counter
	recvBytes     metric.Int64Counter
	recvWireBytes metric.Int64Counter
	compSizeHisto metric.Int64Histogram
}

var _ Interface = &NetworkReporter{}

// SizesStruct is used to pass uncompressed on-wire message lengths to
// the CountSend() and CountReceive() methods.
type SizesStruct struct {
	// Method refers to the gRPC method name
	Method string
	// Length is the uncompressed size
	Length int64
	// WireLength is compressed size
	WireLength int64
}

// Interface describes a *NetworkReporter or a Noop.
type Interface interface {
	// CountSend reports outbound bytes.
	CountSend(ctx context.Context, ss SizesStruct)

	// CountSend reports inbound bytes.
	CountReceive(ctx context.Context, ss SizesStruct)
}

// Noop is a no-op implementation of Interface.
type Noop struct{}

var _ Interface = Noop{}

func (Noop) CountSend(context.Context, SizesStruct)    {}
func (Noop) CountReceive(context.Context, SizesStruct) {}

const (
	bytesUnit           = "bytes"
	sentDescription     = "Number of bytes sent by the component."
	sentWireDescription = "Number of bytes sent on the wire by the component."
	recvDescription     = "Number of bytes received by the component."
	recvWireDescription = "Number of bytes received on the wire by the component."
	compSizeDescription = "Size of compressed payload"
)

// makeSentMetrics builds the sent and sent-wire metric instruments
// for an exporter or receiver using the corresponding `prefix`.
// major` indicates the major direction of the pipeline,
// which is true when sending for exporters, receiving for receivers.
func makeSentMetrics(prefix string, meter metric.Meter, major bool) (sent, sentWire metric.Int64Counter, _ error) {
	var sentBytes metric.Int64Counter = noopmetric.Int64Counter{}
	var err1 error
	if major {
		sentBytes, err1 = meter.Int64Counter(prefix+"_"+SentBytes, metric.WithDescription(sentDescription), metric.WithUnit(bytesUnit))
	}
	sentWireBytes, err2 := meter.Int64Counter(prefix+"_"+SentWireBytes, metric.WithDescription(sentWireDescription), metric.WithUnit(bytesUnit))
	return sentBytes, sentWireBytes, multierr.Append(err1, err2)
}

// makeRecvMetrics builds the received and received-wire metric
// instruments for an exporter or receiver using the corresponding
// `prefix`.  `major` indicates the major direction of the pipeline,
// which is true when sending for exporters, receiving for receivers.
func makeRecvMetrics(prefix string, meter metric.Meter, major bool) (recv, recvWire metric.Int64Counter, _ error) {
	var recvBytes metric.Int64Counter = noopmetric.Int64Counter{}
	var err1 error
	if major {
		recvBytes, err1 = meter.Int64Counter(prefix+"_"+RecvBytes, metric.WithDescription(recvDescription), metric.WithUnit(bytesUnit))
	}
	recvWireBytes, err2 := meter.Int64Counter(prefix+"_"+RecvWireBytes, metric.WithDescription(recvWireDescription), metric.WithUnit(bytesUnit))
	return recvBytes, recvWireBytes, multierr.Append(err1, err2)
}

// NewExporterNetworkReporter creates a new NetworkReporter configured for an exporter.
func NewExporterNetworkReporter(settings exporter.Settings) (*NetworkReporter, error) {
	meter := settings.MeterProvider.Meter(scopeName)
	rep := &NetworkReporter{
		isExporter:    true,
		staticAttr:    attribute.String(ExporterKey, settings.ID.String()),
		compSizeHisto: noopmetric.Int64Histogram{},
	}

	var errors, err error
	rep.compSizeHisto, err = meter.Int64Histogram("otelcol_"+ExporterKey+"_"+CompSize, metric.WithDescription(compSizeDescription), metric.WithUnit(bytesUnit))
	errors = multierr.Append(errors, err)

	rep.sentBytes, rep.sentWireBytes, err = makeSentMetrics("otelcol_"+ExporterKey, meter, true)
	errors = multierr.Append(errors, err)

	// Normally, an exporter counts sent bytes, and skips received
	// bytes.  LevelDetailed will reveal exporter-received bytes.
	rep.recvBytes, rep.recvWireBytes, err = makeRecvMetrics("otelcol_"+ExporterKey, meter, false)
	errors = multierr.Append(errors, err)

	return rep, errors
}

// NewReceiverNetworkReporter creates a new NetworkReporter configured for an exporter.
func NewReceiverNetworkReporter(settings receiver.Settings) (*NetworkReporter, error) {
	meter := settings.MeterProvider.Meter(scopeName)
	rep := &NetworkReporter{
		isExporter:    false,
		staticAttr:    attribute.String(ReceiverKey, settings.ID.String()),
		compSizeHisto: noopmetric.Int64Histogram{},
	}

	var errors, err error
	rep.compSizeHisto, err = meter.Int64Histogram("otelcol_"+ReceiverKey+"_"+CompSize, metric.WithDescription(compSizeDescription), metric.WithUnit(bytesUnit))
	errors = multierr.Append(errors, err)

	rep.recvBytes, rep.recvWireBytes, err = makeRecvMetrics("otelcol_"+ReceiverKey, meter, true)
	errors = multierr.Append(errors, err)

	// Normally, a receiver counts received bytes, and skips sent
	// bytes.  LevelDetailed will reveal receiver-sent bytes.
	rep.sentBytes, rep.sentWireBytes, err = makeSentMetrics("otelcol_"+ReceiverKey, meter, false)
	errors = multierr.Append(errors, err)

	return rep, errors
}

// CountSend is used to report a message sent by the component.  For
// exporters, SizesStruct indicates the size of a request.  For
// receivers, SizesStruct indicates the size of a response.
func (rep *NetworkReporter) CountSend(ctx context.Context, ss SizesStruct) {
	// Indicates basic level telemetry, not counting bytes.
	if rep == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	attrs := metric.WithAttributes(rep.staticAttr, attribute.String("method", ss.Method))

	if ss.Length > 0 {
		if rep.sentBytes != nil {
			rep.sentBytes.Add(ctx, ss.Length, attrs)
		}
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64("sent_uncompressed", ss.Length))
		}
	}
	if ss.WireLength > 0 {
		if rep.isExporter && rep.compSizeHisto != nil {
			rep.compSizeHisto.Record(ctx, ss.WireLength, attrs)
		}
		if rep.sentWireBytes != nil {
			rep.sentWireBytes.Add(ctx, ss.WireLength, attrs)
		}
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64("sent_compressed", ss.WireLength))
		}
	}
}

// CountReceive is used to report a message received by the component.  For
// exporters, SizesStruct indicates the size of a response.  For
// receivers, SizesStruct indicates the size of a request.
func (rep *NetworkReporter) CountReceive(ctx context.Context, ss SizesStruct) {
	// Indicates basic level telemetry, not counting bytes.
	if rep == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	attrs := metric.WithAttributes(rep.staticAttr, attribute.String("method", ss.Method))

	if ss.Length > 0 {
		if rep.recvBytes != nil {
			rep.recvBytes.Add(ctx, ss.Length, attrs)
		}
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64("received_uncompressed", ss.Length))
		}
	}
	if ss.WireLength > 0 {
		if !rep.isExporter && rep.compSizeHisto != nil {
			rep.compSizeHisto.Record(ctx, ss.WireLength, attrs)
		}
		if rep.recvWireBytes != nil {
			rep.recvWireBytes.Add(ctx, ss.WireLength, attrs)
		}
		if span.IsRecording() {
			span.SetAttributes(attribute.Int64("received_compressed", ss.WireLength))
		}
	}
}
