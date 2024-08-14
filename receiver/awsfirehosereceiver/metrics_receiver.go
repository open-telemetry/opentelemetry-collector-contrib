// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

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
	// namePrefixes is a list of attributes that are used to determine
	// the name of the metric.
	namePrefixes []NamePrefixConfig
}

var _ firehoseConsumer = (*metricsConsumer)(nil)

// newMetricsReceiver creates a new instance of the receiver
// with a metricsConsumer.
func newMetricsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]unmarshaler.MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {

	configuredUnmarshaler := unmarshalers[config.RecordType]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedRecordType
	}

	mc := &metricsConsumer{
		consumer:     nextConsumer,
		unmarshaler:  configuredUnmarshaler,
		namePrefixes: config.NamePrefixes,
	}

	return &firehoseReceiver{
		settings: set,
		config:   config,
		consumer: mc,
	}, nil
}

// Consume uses the configured unmarshaler to deserialize the records into a
// single pmetric.Metrics. If there are common attributes available, then it will
// attach those to each of the pcommon.Resources. It will send the final result
// to the next consumer.
func (mc *metricsConsumer) Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) (int, error) {
	md, err := mc.unmarshaler.Unmarshal(records)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if commonAttributes != nil {
		applyCommonAttributes(md, commonAttributes)
	}

	if len(mc.namePrefixes) > 0 {
		applyNamePrefixes(md, mc.namePrefixes)
	}

	err = mc.consumer.ConsumeMetrics(ctx, md)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func applyCommonAttributes(md pmetric.Metrics, commonAttributes map[string]string) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for k, v := range commonAttributes {
			if _, found := rm.Resource().Attributes().Get(k); !found {
				rm.Resource().Attributes().PutStr(k, v)
			}
		}
	}
}

func applyNamePrefixes(md pmetric.Metrics, namePrefixes []NamePrefixConfig) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				m := ilm.Metrics().At(k)
				newName := makePrefixedName(namePrefixes, m.Name(), rm.Resource().Attributes())
				m.SetName(newName)
			}
		}
	}
}

func makePrefixedName(namePrefixes []NamePrefixConfig, name string, ra pcommon.Map) string {
	parts := make([]string, 0, len(namePrefixes)+1)

	for _, np := range namePrefixes {
		attrVal, found := ra.Get(np.AttributeName)
		if !found {
			parts = append(parts, np.Default)
			continue
		}
		s := attrVal.AsString()
		if s == "" {
			parts = append(parts, np.Default)
			continue
		}
		parts = append(parts, sanitizeValue(s))
	}

	parts = append(parts, name)
	return strings.Join(parts, ".")
}

// replace all non-alphanumeric characters with underscores, other than _ and -.
func sanitizeValue(value string) string {
	s := strings.Map(func(r rune) rune {
		if r == '_' || r == '-' {
			return r
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, value)
	// replace runs of underscores with a single underscore
	s = strings.ReplaceAll(s, "__", "_")
	// trim leading and trailing underscores
	return strings.Trim(s, "_")
}
