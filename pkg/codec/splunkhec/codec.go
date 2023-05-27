// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhec // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec/splunkhec"

import (
	"bytes"
	"errors"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	encoding = "splunkhec"
)

var (
	ErrDecodeJSON   = errors.New("error deserializing event")
	ErrRequiredBody = errors.New("required event field missing")
	ErrBlankBody    = errors.New("event field is blank")
	ErrNestedJSON   = errors.New("event contains nested JSON object")
	ErrIsMetric     = errors.New("event contains metric data")
	ErrIsNotMetric  = errors.New("event does not contain metric data")
)

type LogCodec struct {
	logger *zap.Logger
	config *splunk.HecToOtelAttrs
}

func NewLogCodec(logger *zap.Logger, config *splunk.HecToOtelAttrs) *LogCodec {
	return &LogCodec{
		logger: logger,
		config: config,
	}
}

func (c *LogCodec) Unmarshal(b []byte) (plog.Logs, error) {
	dec := jsoniter.NewDecoder(bytes.NewReader(b))
	l := plog.NewLogs()

	var events []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			c.logger.Debug("error deserializing event", zap.Error(err))
			return l, ErrDecodeJSON
		}
		if msg.Event == nil {
			return l, ErrRequiredBody
		}

		if msg.Event == "" {
			return l, ErrBlankBody
		}

		for _, v := range msg.Fields {
			if !isFlatJSONField(v) {
				return l, ErrNestedJSON
			}
		}
		if msg.IsMetric() {
			return l, ErrIsMetric
		}
		events = append(events, &msg)
	}

	return splunkHecToLogData(c.logger, events, c.config)
}

func (c *LogCodec) Encoding() string {
	return encoding
}

type MetricCodec struct {
	logger *zap.Logger
	config *splunk.HecToOtelAttrs
}

func NewMetricCodec(logger *zap.Logger, config *splunk.HecToOtelAttrs) *MetricCodec {
	return &MetricCodec{
		logger: logger,
		config: config,
	}
}

func (c *MetricCodec) Unmarshal(b []byte) (pmetric.Metrics, error) {
	dec := jsoniter.NewDecoder(bytes.NewReader(b))
	l := pmetric.NewMetrics()

	var events []*splunk.Event

	for dec.More() {
		var msg splunk.Event
		err := dec.Decode(&msg)
		if err != nil {
			c.logger.Debug("error deserializing event", zap.Error(err))
			return l, ErrDecodeJSON
		}
		if msg.Event == nil {
			return l, ErrRequiredBody
		}

		if msg.Event == "" {
			return l, ErrBlankBody
		}

		for _, v := range msg.Fields {
			if !isFlatJSONField(v) {
				return l, ErrNestedJSON
			}
		}
		if !msg.IsMetric() {
			return l, ErrIsNotMetric
		}
		events = append(events, &msg)
	}

	return splunkHecToMetricsData(c.logger, events, c.config), nil
}

func (c *MetricCodec) Encoding() string {
	return encoding
}

func isFlatJSONField(field interface{}) bool {
	switch value := field.(type) {
	case map[string]interface{}:
		return false
	case []interface{}:
		for _, v := range value {
			switch v.(type) {
			case map[string]interface{}, []interface{}:
				return false
			}
		}
	}
	return true
}
