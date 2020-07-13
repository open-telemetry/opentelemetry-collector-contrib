package query

import (
	"fmt"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/utils"
	"go.uber.org/zap"
)

// Struct keeps track of info required at a level of recursion
// An instance of this struct can be thought of as a datapoint
// collector for a particular aggregation
type metricCollector struct {
	aggName  string
	aggRes   aggregationResponse
	aggsMeta map[string]*AggregationMeta
	labels   map[string]string
	logger   *zap.Logger
}

// Returns aggregation type
func (dpC *metricCollector) getType() string {
	return dpC.aggsMeta[dpC.aggName].Type
}

// Walks through the response, collecting dimensions and datapoints depending on the
// type of aggregation at each recursive level
func CollectMetrics(resBody HTTPResponse, aggsMeta map[string]*AggregationMeta, labels map[string]string, logger *zap.Logger) []*metricspb.Metric {
	out := make([]*metricspb.Metric, 0)
	aggsResult := resBody.Aggregations

	for k, v := range aggsResult {
		// each aggregation at the highest level starts with an empty set of dimensions
		out = append(out, (&metricCollector{
			aggName:  k,
			aggRes:   *v,
			aggsMeta: aggsMeta,
			labels:   labels,
			logger: logger.With(
				zap.String("aggregation_name", k),
				zap.String("aggregation_type", aggsMeta[k].Type),
			),
		}).recursivelyCollectMetrics()...)
	}

	return out
}

func (dpC *metricCollector) recursivelyCollectMetrics() []*metricspb.Metric {
	sfxMetrics := make([]*metricspb.Metric, 0)

	// The absence of "doc_count" and "buckets" field is a good indicator that
	// the aggregation is a metric aggregation
	if isMetricAggregation(&dpC.aggRes) {
		return dpC.collectMetricsFromMetricAggregation()
	}

	// Recursively collect all datapoints from buckets at this level
	for _, b := range dpC.aggRes.Buckets {
		key, ok := b.Key.(string)

		if !ok {
			dpC.logger.Warn("Found non string key for bucket. Skipping current aggregation and sub aggregations")
			break
		}

		// Pick the current bucket's key as a dimension before recursing down to the next level
		labelsForBucket := utils.CloneStringMap(dpC.labels)
		labelsForBucket[dpC.aggName] = key

		// Send document count as metrics when there are no metric aggregations specified
		// under a bucket aggregation and there aren't sub aggregations as well
		if isTerminalBucket(b) {
			sfxMetrics = append(sfxMetrics,
				collectDocCountFromTerminalBucket(b, dpC.aggName, dpC.getType(), labelsForBucket)...)
			continue
		}

		for k, v := range b.SubAggregations {
			sfxMetrics = append(sfxMetrics, (&metricCollector{
				aggName:  k,
				aggRes:   *v,
				aggsMeta: dpC.aggsMeta,
				labels:   labelsForBucket,
				logger: dpC.logger.With(
					zap.String("aggregation_name", k),
					zap.String("aggregation_type", dpC.aggsMeta[k].Type),
				),
			}).recursivelyCollectMetrics()...)
		}
	}

	// Recursively collect datapoints from sub aggregations
	for k, v := range dpC.aggRes.SubAggregations {
		sfxMetrics = append(sfxMetrics, (&metricCollector{
			aggName:  k,
			aggRes:   *v,
			aggsMeta: dpC.aggsMeta,
			labels:   dpC.labels,
			logger: dpC.logger.With(
				zap.String("aggregation_name", k),
				zap.String("aggregation_type", dpC.aggsMeta[k].Type),
			),
		}).recursivelyCollectMetrics()...)
	}

	return sfxMetrics
}

// Collects "doc_count" from a bucket as a SFx datapoint if a bucket aggregation
// does not have sub metric aggregations
func collectDocCountFromTerminalBucket(bucket *bucketResponse, aggName string, aggType string, dims map[string]string) []*metricspb.Metric {
	dimsForBucket := utils.CloneStringMap(dims)
	dimsForBucket["bucket_aggregation_type"] = aggType

	out, ok := collectMetric(fmt.Sprintf("%s.%s", aggName, "doc_count"), *bucket.DocCount, dimsForBucket)

	if !ok {
		return []*metricspb.Metric{}
	}

	return []*metricspb.Metric{out}
}

// Collects datapoints from supported metric aggregations
func (dpC *metricCollector) collectMetricsFromMetricAggregation() []*metricspb.Metric {

	out := make([]*metricspb.Metric, 0)

	// Add metric aggregation name as a dimension
	sfxDimensionsForMetric := utils.CloneStringMap(dpC.labels)
	sfxDimensionsForMetric["metric_aggregation_type"] = dpC.getType()

	aggType := dpC.getType()
	switch aggType {
	case "stats":
		fallthrough
	case "extended_stats":
		out = append(out, getMetricsFromStats(dpC.aggName, &dpC.aggRes, sfxDimensionsForMetric, dpC.logger)...)
	case "percentiles":
		out = append(out, getMetricsFromPercentiles(dpC.aggName, &dpC.aggRes, sfxDimensionsForMetric, dpC.logger)...)
	default:
		metricName := dpC.aggName
		m, ok := collectMetric(metricName, dpC.aggRes.Value, sfxDimensionsForMetric)

		if !ok {
			dpC.logger.Warn("Invalid value found", zap.Reflect("value", dpC.aggRes.Value))
			return out
		}

		out = append(out, m)
	}

	return out
}

// Collect datapoints from "stats" or "extended_stats" metric aggregation
// Extended stats aggregations look like:
// {
//		"count" : 36370,
//		"min" : 0.0,
//		"max" : 100.0,
//		"avg" : 49.98350288699478,
//		"sum" : 1817900.0,
//		"sum_of_squares" : 1.21849642E8,
//		"variance" : 851.9282953459498,
//		"std_deviation" : 29.187810732323687,
//		"std_deviation_bounds" : {
//			"upper" : 108.35912435164215,
//			"lower" : -8.392118577652596
//  	}
// }
// Metric names from this integration will look like "extended_stats.count",
// "extended_stats.min", "extended_stats.std_deviation_bounds.lower" and so on
func getMetricsFromStats(aggName string, aggRes *aggregationResponse, dims map[string]string, logger *zap.Logger) []*metricspb.Metric {
	out := make([]*metricspb.Metric, 0)

	for k, v := range aggRes.OtherValues {
		switch k {
		case "std_deviation_bounds":
			m, ok := v.(map[string]interface{})

			if !ok {
				logger.Warn("Invalid value found for stat", zap.Reflect("value", v), zap.String("extended_stat", k))
				continue
			}

			for bk, bv := range m {
				metricName := fmt.Sprintf("%s.%s.%s", aggName, k, bk)
				dp, ok := collectMetric(metricName, bv, dims)

				if !ok {
					logger.Warn("Invalid value found for stat", zap.Reflect("value", bv), zap.String("stat", k))
					continue
				}

				out = append(out, dp)
			}
		default:
			metricName := fmt.Sprintf("%s.%s", aggName, k)
			dp, ok := collectMetric(metricName, v, dims)

			if !ok {
				logger.Warn("Invalid value found for stat", zap.Reflect("value", v), zap.String("stat", k))
				continue
			}

			out = append(out, dp)
		}
	}

	return out
}

// Collect datapoint from "percentiles" metric aggregation
func getMetricsFromPercentiles(aggName string, aggRes *aggregationResponse, dims map[string]string, logger *zap.Logger) []*metricspb.Metric {
	out := make([]*metricspb.Metric, 0)

	// Values are always expected to be a map between the percentile and the
	// actual value itself of the metric
	values, ok := aggRes.Values.(map[string]interface{})

	if !ok {
		logger.Warn("No valid values found in percentiles aggregation")
	}

	// Metric name is constituted of the aggregation type "percentiles" and the actual percentile
	// Metric names from this aggregation will look like "percentiles.p99", "percentiles.p50" and
	// the aggregation name used to compute the metric will be a sent in as "metric_aggregation_name"
	// dimension on the datapoint
	for k, v := range values {
		p, err := strconv.ParseFloat(k, 64)

		if err != nil {
			logger.Warn("Invalid percentile found", zap.String("percentile", k))
			continue
		}

		// Remove trailing zeros
		metricName := fmt.Sprintf("%s.p%s", aggName, strconv.FormatFloat(p, 'f', -1, 64))
		dp, ok := collectMetric(metricName, v, dims)

		if !ok {
			logger.Warn("Invalid value for percentile found", zap.String("percentile", k))
			continue
		}

		out = append(out, dp)
	}

	return out
}

// Returns true if aggregation is a metric aggregation
func isMetricAggregation(aggRes *aggregationResponse) bool {
	return aggRes.DocCount == nil && len(aggRes.Buckets) == 0
}

// Returns true if bucket aggregation is at the deepest level without
// sub metric aggregations
func isTerminalBucket(b *bucketResponse) bool {
	return len(b.SubAggregations) == 0 && b.DocCount != nil
}

// Collects a single datapoint from an interface, returns false if no datapoint can be derived
func collectMetric(metricName string, value interface{}, labels map[string]string) (*metricspb.Metric, bool) {
	switch v := value.(type) {
	case float64:
		return utils.PrepareGaugeF(metricName, labels, &v), true
	case int64:
		return utils.PrepareGauge(metricName, labels, &v), true
	case *float64:
		return utils.PrepareGaugeF(metricName, labels, v), true
	case *int64:
		return utils.PrepareGauge(metricName, labels, v), true
	default:
		return nil, false
	}
}
