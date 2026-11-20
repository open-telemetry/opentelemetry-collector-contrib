// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type promLogger struct {
	realLog *zap.Logger
}

func newPromLogger(zapLog *zap.Logger) *promLogger {
	return &promLogger{
		realLog: zapLog,
	}
}

// classifyGatherError maps a Prometheus Registry.Gather error to a fixed
// allow-listed category so diagnostic logs never expose raw dto.Metric values.
func classifyGatherError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "was collected before with the same name and label values"):
		return "duplicate_metric"
	case strings.Contains(msg, "collides with previously collected"):
		return "suffix_collision"
	case strings.Contains(msg, "has help"):
		return "help_conflict"
	case strings.Contains(msg, "has two or more labels with the same name"),
		strings.Contains(msg, "duplicate label names"):
		return "duplicate_label"
	case strings.Contains(msg, "not a valid metric name"):
		return "invalid_metric_name"
	case strings.Contains(msg, "invalid name"),
		strings.Contains(msg, "not a valid label name"),
		strings.Contains(msg, "not utf8"),
		strings.Contains(msg, "not valid UTF-8"),
		strings.Contains(msg, `explicit "quantile" label`),
		strings.Contains(msg, "inconsistent with descriptor"):
		return "invalid_label"
	case strings.Contains(msg, "should be a "), strings.Contains(msg, "is not a "):
		return "type_conflict"
	case strings.Contains(msg, "panic recovered"):
		return "collector_panic"
	default:
		return "unknown"
	}
}

func classifyGatherErrors(err error) (int64, []string) {
	if err == nil {
		return 0, nil
	}
	if multiErr, ok := errors.AsType[prometheus.MultiError](err); ok {
		reasons := make([]string, 0, len(multiErr))
		for _, subErr := range multiErr {
			reasons = append(reasons, classifyGatherError(subErr))
		}
		slices.Sort(reasons)
		return int64(len(multiErr)), slices.Compact(reasons)
	}
	return 1, []string{classifyGatherError(err)}
}

func classifyEncodeError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, syscall.EPIPE), errors.Is(err, syscall.ECONNRESET), errors.Is(err, io.ErrClosedPipe):
		return "client_disconnected"
	default:
		return "encode_error"
	}
}

// Println logs promhttp handler errors at Debug level without formatting raw
// Prometheus dto.Metric values or processed label values.
func (l *promLogger) Println(v ...any) {
	if !l.realLog.Core().Enabled(zap.DebugLevel) || len(v) == 0 {
		return
	}
	msg, ok := v[0].(string)
	if !ok {
		l.realLog.Debug("error in Prometheus HTTP handler")
		return
	}
	switch msg {
	case "error gathering metrics:":
		var errCount int64 = 1
		var reasons []string
		if len(v) > 1 {
			if err, ok := v[1].(error); ok {
				errCount, reasons = classifyGatherErrors(err)
			}
		}
		l.realLog.Debug("error gathering Prometheus metrics",
			zap.Int64("error_count", errCount),
			zap.Strings("reasons", reasons),
		)
	case "error encoding and sending metric family:":
		reason := "encode_error"
		if len(v) > 1 {
			if err, ok := v[1].(error); ok {
				reason = classifyEncodeError(err)
			}
		}
		l.realLog.Debug("error encoding and sending Prometheus metric family",
			zap.String("reason", reason),
		)
	case "error getting writer":
		if len(v) > 1 {
			if err, ok := v[1].(error); ok {
				l.realLog.Debug("error creating Prometheus response writer", zap.Error(err))
				return
			}
		}
		l.realLog.Debug("error creating Prometheus response writer")
	default:
		l.realLog.Debug("error in Prometheus HTTP handler", zap.String("cause", msg))
	}
}
