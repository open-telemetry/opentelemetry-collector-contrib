// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"context"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var (
	tagGrpcStatusCode, _     = tag.NewKey("grpc_response_code")
	tagHTTPStatusCode, _     = tag.NewKey("http_status_code")
	tagRequestUserAgent, _   = tag.NewKey("user_agent")
	tagAPIKey, _             = tag.NewKey("api_key")
	tagDataType, _           = tag.NewKey("data_type")
	tagMetricType, _         = tag.NewKey("metric_type")
	tagMetricTemporality, _  = tag.NewKey("metric_temporality")
	tagHasSpanEvents, _      = tag.NewKey("has_span_events")
	tagHasSpanLinks, _       = tag.NewKey("has_span_links")
	tagAttributeLocation, _  = tag.NewKey("attribute_location")
	tagAttributeValueType, _ = tag.NewKey("attribute_type")
	tagKeys                  = []tag.Key{tagGrpcStatusCode, tagHTTPStatusCode, tagRequestUserAgent, tagAPIKey, tagDataType}
	metricMetadataTagKeys    = []tag.Key{tagGrpcStatusCode, tagHTTPStatusCode, tagRequestUserAgent, tagAPIKey, tagDataType, tagMetricType, tagMetricTemporality}
	spanMetadataTagKeys      = []tag.Key{tagGrpcStatusCode, tagHTTPStatusCode, tagRequestUserAgent, tagAPIKey, tagDataType, tagHasSpanEvents, tagHasSpanLinks}
	attributeMetadataTagKeys = []tag.Key{tagGrpcStatusCode, tagHTTPStatusCode, tagRequestUserAgent, tagAPIKey, tagDataType, tagAttributeLocation, tagAttributeValueType}

	statRequestCount         = stats.Int64("newrelicexporter_request_count", "Number of requests processed", stats.UnitDimensionless)
	statInputDatapointCount  = stats.Int64("newrelicexporter_input_datapoint_count", "Number of data points received by the exporter.", stats.UnitDimensionless)
	statOutputDatapointCount = stats.Int64("newrelicexporter_output_datapoint_count", "Number of data points sent to the HTTP API", stats.UnitDimensionless)
	statExporterTime         = stats.Int64("newrelicexporter_exporter_time", "Wall clock time (milliseconds) spent in the exporter", stats.UnitMilliseconds)
	statExternalTime         = stats.Int64("newrelicexporter_external_time", "Wall clock time (milliseconds) spent sending data to the HTTP API", stats.UnitMilliseconds)
	statMetricMetadata       = stats.Int64("newrelicexporter_metric_metadata_count", "Number of metrics processed", stats.UnitDimensionless)
	statSpanMetadata         = stats.Int64("newrelicexporter_span_metadata_count", "Number of spans processed", stats.UnitDimensionless)
	statAttributeMetadata    = stats.Int64("newrelicexporter_attribute_metadata_count", "Number of attributes processed", stats.UnitDimensionless)
)

const EuKeyPrefix = "eu01xx"

// MetricViews return metric views for Kafka receiver.
func MetricViews() []*view.View {
	return []*view.View{
		buildView(tagKeys, statRequestCount, view.Sum()),
		{
			Name:        "newrelicexporter_input_datapoint_count_notag",
			Measure:     statInputDatapointCount,
			Description: statInputDatapointCount.Description(),
			TagKeys:     []tag.Key{},
			Aggregation: view.Sum(),
		},
		buildView(tagKeys, statOutputDatapointCount, view.Sum()),
		{
			Name:        "newrelicexporter_output_datapoint_count_notag",
			Measure:     statOutputDatapointCount,
			Description: statOutputDatapointCount.Description(),
			TagKeys:     []tag.Key{},
			Aggregation: view.Sum(),
		},
		buildView(tagKeys, statExporterTime, view.Sum()),
		buildView(tagKeys, statExternalTime, view.Sum()),
		buildView(metricMetadataTagKeys, statMetricMetadata, view.Sum()),
		buildView(spanMetadataTagKeys, statSpanMetadata, view.Sum()),
		buildView(attributeMetadataTagKeys, statAttributeMetadata, view.Sum()),
	}
}

func buildView(tagKeys []tag.Key, m stats.Measure, a *view.Aggregation) *view.View {
	return &view.View{
		Name:        m.Name(),
		Measure:     m,
		Description: m.Description(),
		TagKeys:     tagKeys,
		Aggregation: a,
	}
}

type metricStatsKey struct {
	MetricType        pdata.MetricDataType
	MetricTemporality pdata.MetricAggregationTemporality
}

type spanStatsKey struct {
	hasEvents bool
	hasLinks  bool
}

type attributeLocation int

const (
	attributeLocationResource attributeLocation = iota
	attributeLocationSpan
	attributeLocationSpanEvent
	attributeLocationLog
)

func (al attributeLocation) String() string {
	switch al {
	case attributeLocationResource:
		return "resource"
	case attributeLocationSpan:
		return "span"
	case attributeLocationSpanEvent:
		return "span_event"
	case attributeLocationLog:
		return "log"
	}
	return ""
}

type attributeStatsKey struct {
	location      attributeLocation
	attributeType pdata.AttributeValueType
}

type exportMetadata struct {
	// Metric tags
	grpcResponseCode codes.Code // The gRPC response code
	httpStatusCode   int        // The HTTP response status code form the HTTP API
	apiKey           string     // The API key from the request
	userAgent        string     // The User-Agent from the request
	dataType         string     // The type of data being recorded

	// Metric values
	dataInputCount         int                       // Number of resource spans in the request
	dataOutputCount        int                       // Number of spans sent to the trace API
	exporterTime           time.Duration             // Total time spent in the newrelic exporter
	externalDuration       time.Duration             // Time spent sending to the trace API
	metricMetadataCount    map[metricStatsKey]int    // Number of metrics by type and temporality
	spanMetadataCount      map[spanStatsKey]int      // Number of spans by whether or not they have events or links
	attributeMetadataCount map[attributeStatsKey]int // Number of attributes by location and type
}

func newTraceMetadata(ctx context.Context) exportMetadata {
	return initMetadata(ctx, "trace")
}

func newLogMetadata(ctx context.Context) exportMetadata {
	return initMetadata(ctx, "log")
}

func newMetricMetadata(ctx context.Context) exportMetadata {
	return initMetadata(ctx, "metric")
}

func initMetadata(ctx context.Context, dataType string) exportMetadata {
	userAgent := "not_present"
	if md, ctxOk := metadata.FromIncomingContext(ctx); ctxOk {
		if values, headerOk := md["user-agent"]; headerOk {
			userAgent = values[0]
		}
	}

	return exportMetadata{
		userAgent:              userAgent,
		apiKey:                 "not_present",
		dataType:               dataType,
		metricMetadataCount:    make(map[metricStatsKey]int, 8*3 /* 8 metric types by 3 temporarilities */),
		spanMetadataCount:      make(map[spanStatsKey]int, 2*2 /* combinations of the 2 bool key values */),
		attributeMetadataCount: make(map[attributeStatsKey]int, 3*7 /* spans can have 7 value types in 4 different locations */),
	}
}

func (d exportMetadata) recordMetrics(ctx context.Context) error {
	tags := []tag.Mutator{
		tag.Insert(tagGrpcStatusCode, d.grpcResponseCode.String()),
		tag.Insert(tagHTTPStatusCode, strconv.Itoa(d.httpStatusCode)),
		tag.Insert(tagRequestUserAgent, d.userAgent),
		tag.Insert(tagAPIKey, d.apiKey),
		tag.Insert(tagDataType, d.dataType),
	}

	var errs error
	errs = multierr.Append(errs, stats.RecordWithTags(ctx, tags,
		statRequestCount.M(1),
		statInputDatapointCount.M(int64(d.dataInputCount)),
		statOutputDatapointCount.M(int64(d.dataOutputCount)),
		statExporterTime.M(d.exporterTime.Milliseconds()),
		statExternalTime.M(d.externalDuration.Milliseconds()),
	))

	if len(d.metricMetadataCount) > 0 {
		metricMetadataTagMutators := make([]tag.Mutator, len(tags)+2)
		copy(metricMetadataTagMutators, tags)
		for k, v := range d.metricMetadataCount {
			metricTypeTag := tag.Insert(tagMetricType, k.MetricType.String())
			metricMetadataTagMutators[len(metricMetadataTagMutators)-2] = metricTypeTag

			temporalityTag := tag.Insert(tagMetricTemporality, k.MetricTemporality.String())
			metricMetadataTagMutators[len(metricMetadataTagMutators)-1] = temporalityTag

			errs = multierr.Append(errs, stats.RecordWithTags(ctx, metricMetadataTagMutators, statMetricMetadata.M(int64(v))))
		}
	}

	if len(d.spanMetadataCount) > 0 {
		spanMetadataTagMutators := make([]tag.Mutator, len(tags)+2)
		copy(spanMetadataTagMutators, tags)
		for k, v := range d.spanMetadataCount {
			hasSpanEventsTag := tag.Insert(tagHasSpanEvents, strconv.FormatBool(k.hasEvents))
			spanMetadataTagMutators[len(spanMetadataTagMutators)-2] = hasSpanEventsTag

			hasSpanLinksTag := tag.Insert(tagHasSpanLinks, strconv.FormatBool(k.hasLinks))
			spanMetadataTagMutators[len(spanMetadataTagMutators)-1] = hasSpanLinksTag

			errs = multierr.Append(errs, stats.RecordWithTags(ctx, spanMetadataTagMutators, statSpanMetadata.M(int64(v))))
		}
	}

	if len(d.attributeMetadataCount) > 0 {
		attributeMetadataMutators := make([]tag.Mutator, len(tags)+2)
		copy(attributeMetadataMutators, tags)
		for k, v := range d.attributeMetadataCount {
			locationTag := tag.Insert(tagAttributeLocation, k.location.String())
			attributeMetadataMutators[len(attributeMetadataMutators)-2] = locationTag

			typeTag := tag.Insert(tagAttributeValueType, k.attributeType.String())
			attributeMetadataMutators[len(attributeMetadataMutators)-1] = typeTag

			errs = multierr.Append(errs, stats.RecordWithTags(ctx, attributeMetadataMutators, statAttributeMetadata.M(int64(v))))
		}
	}

	return errs
}

func sanitizeAPIKeyForLogging(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey
	}
	end := 8
	if strings.HasPrefix(apiKey, EuKeyPrefix) {
		end += len(EuKeyPrefix)
	}
	return apiKey[:end]
}
