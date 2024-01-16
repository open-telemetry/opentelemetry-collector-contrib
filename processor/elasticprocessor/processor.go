package elasticconnector

import (
	"context"
	"strings"
	
	"github.com/tommyers-elastic/opentelemetry-collector-contrib/processor/elasticprocessor/internal/datastream"
	"github.com/tommyers-elastic/opentelemetry-collector-contrib/processor/elasticprocessor/internal/hostmetrics"
	
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	
	"go.uber.org/zap"
)

type ElasticProcessor struct {
	logger          *zap.Logger
}

func newProcessor(set processor.CreateSettings) *ElasticProcessor {
	return &ElasticProcessor{set.Logger}
}

func (p *ElasticProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetric := md.ResourceMetrics().At(i)

		for j := 0; j < resourceMetric.ScopeMetrics().Len(); j++ {
			scopeMetric := resourceMetric.ScopeMetrics().At(j)
			scope := scopeMetric.Scope()

			if err := datastream.AddDataStreamFields(datastream.Metrics, scope); err != nil {
				p.logger.Error("error adding Elastic data stream fields", zap.Error(err))
			}

			if strings.HasPrefix(scope.Name(), "otelcol/hostmetricsreceiver") {
				if err := hostmetrics.TransformHostMetricsForElasticCompatibilty(scopeMetric); err != nil {
					p.logger.Error("error adding hostmetrics data", zap.Error(err))
				}
			}
		}
	}
	
	return md, nil
}

func (p *ElasticProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		resourceLog := ld.ResourceLogs().At(i)

		for j := 0; j < resourceLog.ScopeLogs().Len(); j++ {
			scopeMetric := resourceLog.ScopeLogs().At(j)
			scope := scopeMetric.Scope()

			if err := datastream.AddDataStreamFields(datastream.Logs, scope); err != nil {
				p.logger.Error("error adding Elastic data stream fields", zap.Error(err))
			}
		}
	}

	return ld, nil
}

func (p *ElasticProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, nil
}