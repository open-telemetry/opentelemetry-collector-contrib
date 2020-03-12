package redisreceiver

import (
	resourceProto "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
)

type metricsService struct {
	apiParser *apiParser
}

func newMetricsService(client client) *metricsService {
	return &metricsService{apiParser: newApiParser(client)}
}

func (s *metricsService) getMetricsData(redisMetrics []*redisMetric) (*consumerdata.MetricsData, error) {
	redisInfo, err := s.apiParser.info()
	if err != nil {
		return nil, err
	}
	protoMetrics, err := buildProtoMetrics(redisInfo, redisMetrics)
	if err != nil {
		return nil, err
	}
	return &consumerdata.MetricsData{
		Resource: &resourceProto.Resource{
			Type:   typeStr,
			Labels: map[string]string{"type": typeStr},
		},
		Metrics: protoMetrics,
	}, nil
}
