// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanmetricsprocessor

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/metricsdimensions"
)

const (
	serviceNameKey = conventions.AttributeServiceName
	operationKey   = "operation" // is there a constant we can refer to?
	spanKindKey    = tracetranslator.TagSpanKind
	statusCodeKey  = tracetranslator.TagStatusCode
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = float64(maxDuration.Milliseconds())

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
	mandatoryDimensions = []string{serviceNameKey, operationKey, spanKindKey, statusCodeKey}
)

type processorImp struct {
	lock   sync.RWMutex
	logger *zap.Logger
	config Config

	metricsExporter component.MetricsExporter
	nextConsumer    consumer.TracesConsumer

	// Additional dimensions to add to metrics.
	dimensions []Dimension

	// Call & Error counts.
	callSum map[string]int64

	// Latency histogram.
	latencyCount        map[string]uint64
	latencySum          map[string]float64
	latencyBucketCounts map[string][]uint64
	latencyBounds       []float64

	jsonSerder JSONSerder

	metricsKeyCache *metricsdimensions.Cache
}

func newProcessor(logger *zap.Logger, config configmodels.Exporter, nextConsumer consumer.TracesConsumer) *processorImp {
	logger.Info("Building spanmetricsprocessor")
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets, func(duration time.Duration) float64 {
			return float64(duration.Milliseconds())
		})

		// "Catch-all" bucket.
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	return &processorImp{
		logger:              logger,
		config:              *pConfig,
		callSum:             make(map[string]int64),
		latencyBounds:       bounds,
		latencySum:          make(map[string]float64),
		latencyCount:        make(map[string]uint64),
		latencyBucketCounts: make(map[string][]uint64),
		nextConsumer:        nextConsumer,
		dimensions:          pConfig.Dimensions,
		jsonSerder:          &JSONSerde{},
		metricsKeyCache:     metricsdimensions.NewCache(),
	}
}

func mapDurationsToMillis(vs []time.Duration, f func(duration time.Duration) float64) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// Start implements the component.Component interface.
func (p *processorImp) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting spanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[configmodels.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.Name())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.Name())

		p.logger.Info("Looking for spanmetrics exporter from available exporters",
			zap.String("spanmetrics-exporter", p.config.MetricsExporter),
			zap.Any("available-exporters", availableMetricsExporters),
		)
		if k.Name() == p.config.MetricsExporter {
			p.metricsExporter = metricsExp
			p.logger.Info("Found exporter", zap.String("spanmetrics-exporter", p.config.MetricsExporter))
			break
		}
	}
	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}
	p.logger.Info("Started spanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down spanmetricsprocessor")
	return nil
}

// GetCapabilities implements the component.Processor interface.
func (p *processorImp) GetCapabilities() component.ProcessorCapabilities {
	p.logger.Info("GetCapabilities for spanmetricsprocessor")
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

// ConsumeTraces implements the consumer.TracesConsumer interface.
// It aggregates the trace data to generate metrics, forwarding these metrics to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.logger.Info("Consuming trace data")

	if err := p.aggregateMetrics(traces); err != nil {
		return err
	}

	m, err := p.buildMetrics()
	if err != nil {
		return err
	}

	// Firstly, export metrics to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, *m); err != nil {
		return err
	}

	// Forward trace data unmodified.
	if err := p.nextConsumer.ConsumeTraces(ctx, traces); err != nil {
		return err
	}

	return nil
}

// buildMetrics collects the computed raw metrics data, builds the metrics object and
// writes the raw metrics data into the metrics object.
func (p *processorImp) buildMetrics() (*pdata.Metrics, error) {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	ilm.InstrumentationLibrary().SetName("spanmetricsprocessor")

	p.lock.RLock()
	if err := p.collectCallMetrics(&ilm); err != nil {
		return nil, err
	}
	if err := p.collectLatencyMetrics(&ilm); err != nil {
		return nil, err
	}
	p.lock.RUnlock()

	rm := pdata.NewResourceMetrics()
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Append(ilm)

	m := pdata.NewMetrics()
	m.ResourceMetrics().Append(rm)
	return &m, nil
}

// collectLatencyMetrics collects the raw latency metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectLatencyMetrics(ilm *pdata.InstrumentationLibraryMetrics) error {
	for key := range p.latencyCount {
		dpLatency := pdata.NewIntHistogramDataPoint()
		dpLatency.SetTimestamp(pdata.TimeToUnixNano(time.Now()))
		dpLatency.SetExplicitBounds(p.latencyBounds)
		dpLatency.SetBucketCounts(p.latencyBucketCounts[key])
		dpLatency.SetCount(p.latencyCount[key])
		dpLatency.SetSum(int64(p.latencySum[key]))
		kmap := make(map[string]string)
		if err := p.jsonSerder.Unmarshal([]byte(key), &kmap); err != nil {
			return err
		}
		dpLatency.LabelsMap().InitFromMap(kmap)

		mLatency := pdata.NewMetric()
		mLatency.SetDataType(pdata.MetricDataTypeIntHistogram)
		mLatency.SetName("latency")
		mLatency.IntHistogram().DataPoints().Append(dpLatency)
		mLatency.IntHistogram().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		ilm.Metrics().Append(mLatency)
	}
	return nil
}

// collectCallMetrics collects the raw call count metrics, writing the data
// into the given instrumentation library metrics.
func (p *processorImp) collectCallMetrics(ilm *pdata.InstrumentationLibraryMetrics) error {
	for key := range p.callSum {
		dpCalls := pdata.NewIntDataPoint()
		dpCalls.SetTimestamp(pdata.TimeToUnixNano(time.Now()))
		dpCalls.SetValue(p.callSum[key])

		kmap := make(map[string]string)
		if err := p.jsonSerder.Unmarshal([]byte(key), &kmap); err != nil {
			return err
		}
		dpCalls.LabelsMap().InitFromMap(kmap)

		mCalls := pdata.NewMetric()
		mCalls.SetDataType(pdata.MetricDataTypeIntSum)
		mCalls.SetName("calls")
		mCalls.IntSum().DataPoints().Append(dpCalls)
		mCalls.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		ilm.Metrics().Append(mCalls)
	}
	return nil
}

// aggregateMetrics aggregates the raw metrics from the input trace data.
// Each metric is identified by a key that is built from the service name
// and span metadata such as operation, kind, status_code and any additional
// dimensions the user has configured.
func (p *processorImp) aggregateMetrics(traces pdata.Traces) error {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rspans := traces.ResourceSpans().At(i)
		r := rspans.Resource()

		attr, ok := r.Attributes().Get(conventions.AttributeServiceName)
		if !ok {
			continue
		}
		serviceName := attr.StringVal()
		if err := p.aggregateMetricsForServiceSpans(rspans, serviceName); err != nil {
			return err
		}
	}
	return nil
}

func (p *processorImp) aggregateMetricsForServiceSpans(rspans pdata.ResourceSpans, serviceName string) error {
	ilsSlice := rspans.InstrumentationLibrarySpans()
	for j := 0; j < ilsSlice.Len(); j++ {
		ils := ilsSlice.At(j)
		spans := ils.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			if err := p.aggregateMetricsForSpan(serviceName, span); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *processorImp) aggregateMetricsForSpan(serviceName string, span pdata.Span) error {
	key, err := p.buildKey(serviceName, span)
	if err != nil {
		return err
	}

	p.lock.Lock()
	p.updateCallMetrics(key)
	p.lock.Unlock()

	latency := float64(span.EndTime()-span.StartTime()) / float64(time.Millisecond.Nanoseconds())

	// Binary search to find the latency bucket index.
	index := sort.SearchFloat64s(p.latencyBounds, latency)

	p.lock.Lock()
	p.updateLatencyMetrics(key, latency, index)
	p.lock.Unlock()
	return nil
}

// updateCallMetrics increments the call count for the given metric key.
func (p *processorImp) updateCallMetrics(key string) {
	p.callSum[key]++
}

// updateLatencyMetrics increments the histogram counts for the given metric key and bucket index.
func (p *processorImp) updateLatencyMetrics(key string, latency float64, index int) {
	if _, ok := p.latencyBucketCounts[key]; !ok {
		p.latencyBucketCounts[key] = make([]uint64, len(p.latencyBounds))
	}
	p.latencySum[key] += latency
	p.latencyCount[key]++
	p.latencyBucketCounts[key][index]++
}

// buildKey builds the metric key from the service name and span metadata
// such as operation, kind, status_code and any additional dimensions the
// user has configured.
func (p *processorImp) buildKey(serviceName string, span pdata.Span) (string, error) {
	d := p.getDimensions(serviceName, span)
	p.lock.Lock()
	lastDimension := p.metricsKeyCache.InsertDimensions(d...)
	p.lock.Unlock()

	// Found a cached key, just return it.
	if lastDimension.HasCachedMetricKey() {
		return lastDimension.GetCachedMetricKey(), nil
	}

	keyMap := make(map[string]string)
	for _, kv := range d {
		keyMap[kv.Key] = kv.Value
	}

	// Otherwise, serialize the map to JSON.
	kjson, err := p.jsonSerder.Marshal(keyMap)
	if err != nil {
		return "", err
	}

	kjsonstr := string(kjson)
	lastDimension.SetCachedMetricKey(kjsonstr)
	return kjsonstr, nil
}

// getDimensions returns a list of key-value pairs of dimensions for the given span's metrics.
func (p *processorImp) getDimensions(serviceName string, span pdata.Span) []metricsdimensions.DimensionKeyValue {
	ds := make([]metricsdimensions.DimensionKeyValue, 0, len(mandatoryDimensions)+len(p.dimensions))
	ds = append(ds,
		metricsdimensions.DimensionKeyValue{Key: serviceNameKey, Value: serviceName},
		metricsdimensions.DimensionKeyValue{Key: operationKey, Value: span.Name()},
		metricsdimensions.DimensionKeyValue{Key: spanKindKey, Value: span.Kind().String()},
		metricsdimensions.DimensionKeyValue{Key: statusCodeKey, Value: span.Status().Code().String()},
	)
	spanAttr := span.Attributes()
	var value string
	for _, d := range p.dimensions {
		// Set the default if configured, otherwise this metric will have no value set for the dimension.
		if d.Default != nil {
			value = *d.Default
		}
		// Map the various attribute values to string.
		if attr, ok := spanAttr.Get(d.Name); ok {
			switch attr.Type() {
			case pdata.AttributeValueSTRING:
				value = attr.StringVal()
			case pdata.AttributeValueINT:
				value = strconv.FormatInt(attr.IntVal(), 10)
			case pdata.AttributeValueDOUBLE:
				value = strconv.FormatFloat(attr.DoubleVal(), 'f', -1, 64)
			case pdata.AttributeValueBOOL:
				value = strconv.FormatBool(attr.BoolVal())
			case pdata.AttributeValueNULL:
				value = ""
			default:
				p.logger.Warn("Unsupported tag data type", zap.String("data-type", attr.Type().String()))
				continue
			}
		}
		ds = append(ds, metricsdimensions.DimensionKeyValue{Key: d.Name, Value: value})
	}
	return ds
}
