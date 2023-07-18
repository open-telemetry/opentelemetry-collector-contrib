// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"fmt"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var getAttribute = func(key string) string {
	return fmt.Sprintf("kafka.header.%s", key)
}

type headerExtractor struct {
	logger  *zap.Logger
	headers []string
}

func (ex *headerExtractor) extractHeadersTraces(traces ptrace.Traces, message *sarama.ConsumerMessage) {
	for _, header := range ex.headers {
		value, ok := getHeaderValue(message.Headers, header)
		if !ok {
			ex.logger.Warn("Header key not found in the trace: ", zap.String("key", header))
			continue
		}
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rs := traces.ResourceSpans().At(i)
			rs.Resource().Attributes().PutStr(getAttribute(header), value)
		}
	}
}

func (ex *headerExtractor) extractHeadersLogs(logs plog.Logs, message *sarama.ConsumerMessage) {
	for _, header := range ex.headers {
		value, ok := getHeaderValue(message.Headers, header)
		if !ok {
			ex.logger.Warn("Header key not found in the logger: ", zap.String("key", header))
			continue
		}
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rl := logs.ResourceLogs().At(i)
			rl.Resource().Attributes().PutStr(getAttribute(header), value)
		}
	}
}

func (ex *headerExtractor) extractHeadersMetrics(metrics pmetric.Metrics, message *sarama.ConsumerMessage) {
	for _, header := range ex.headers {
		value, ok := getHeaderValue(message.Headers, header)
		if !ok {
			ex.logger.Warn("Header key not found in the metric: ", zap.String("key", header))
			continue
		}
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			rm := metrics.ResourceMetrics().At(i)
			rm.Resource().Attributes().PutStr(getAttribute(header), value)
		}
	}
}

func getHeaderValue(headers []*sarama.RecordHeader, header string) (string, bool) {
	for _, kafkaHeader := range headers {
		headerKey := string(kafkaHeader.Key)
		if headerKey == header {
			// matching header found
			return string(kafkaHeader.Value), true
		}
	}
	// no header found matching the key, report to the user
	return "", false
}
