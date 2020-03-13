package redisreceiver

import (
	resourceProto "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"go.uber.org/zap"
)

type metricsService struct {
	apiParser *apiParser
	logger    *zap.Logger
}

func newMetricsService(client client, logger *zap.Logger) *metricsService {
	return &metricsService{
		apiParser: newApiParser(client),
		logger:    logger,
	}
}

func (s *metricsService) getMetricsData(redisMetrics []*redisMetric) (*consumerdata.MetricsData, error) {
	redisInfo, err := s.apiParser.info()
	if err != nil {
		return nil, err
	}
	protoMetrics, parsingErrors := buildProtoMetrics(redisInfo, redisMetrics)
	if parsingErrors != nil {
		s.logger.Warn("errors parsing redis string", zap.Errors("parsing errors", parsingErrors))
	}
	return &consumerdata.MetricsData{
		Resource: &resourceProto.Resource{
			Type:   typeStr,
			Labels: map[string]string{"type": typeStr},
		},
		Metrics: protoMetrics,
	}, nil
}
