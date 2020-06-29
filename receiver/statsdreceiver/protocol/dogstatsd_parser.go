package protocol

import (
	"errors"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type DogStatsDParser struct{}

func (p *DogStatsDParser) Parse(line string) (*metricspb.Metric, error) {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return nil, errors.New("not enough statsd message parts")
	}

	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: parts[0],
		},
	}, nil
}
