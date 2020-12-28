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

	jsonSerder JsonSerder
}

func newProcessor(logger *zap.Logger, config configmodels.Exporter, nextConsumer consumer.TracesConsumer) (*processorImp, error) {
	logger.Info("building spanmetricsprocessor")
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
		jsonSerder:          &JsonSerde{},
	}, nil
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
	p.logger.Info("starting spanmetricsprocessor")
	exporters := host.GetExporters()

	var availableMetricsExporters []string

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[configmodels.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a metrics exporter", k.Name())
		}

		availableMetricsExporters = append(availableMetricsExporters, k.Name())

		p.logger.Info("checking if : '" + k.Name() + "' is the configured spanmetrics exporter: '" + p.config.MetricsExporter + "'")
		if k.Name() == p.config.MetricsExporter {
			p.metricsExporter = metricsExp
			p.logger.Info("found exporter: '" + p.config.MetricsExporter + "'")
			break
		}
	}
	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: '%s'; please configure metrics_exporter from one of: %+v",
			p.config.MetricsExporter, availableMetricsExporters)
	}
	p.logger.Info("started spanmetricsprocessor")
	return nil
}

// Shutdown implements the component.Component interface.
func (p *processorImp) Shutdown(ctx context.Context) error {
	p.logger.Info("shutting down spanmetricsprocessor")
	return nil
}

// GetCapabilities implements the component.Processor interface.
func (p *processorImp) GetCapabilities() component.ProcessorCapabilities {
	p.logger.Info("GetCapabilities for spanmetricsprocessor")
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}

// ConsumeTraces implements the consumer.TracesConsumer interface.
// It aggregates the trace data to generate metrics, forwarding these metrics
// to the discovered metrics exporter.
// The original input trace data will be forwarded to the next consumer, unmodified.
func (p *processorImp) ConsumeTraces(ctx context.Context, traces pdata.Traces) error {
	p.logger.Info("consuming trace data")

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

	if err := p.withRLock(func() error {
		if err := p.collectCallMetrics(&ilm); err != nil {
			return err
		}
		return p.collectLatencyMetrics(&ilm)
	}); err != nil {
		return nil, err
	}

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
		ilsSlice := rspans.InstrumentationLibrarySpans()
		for j := 0; j < ilsSlice.Len(); j++ {
			ils := ilsSlice.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				key, err := p.buildKey(serviceName, span)
				if err != nil {
					return err
				}

				p.withLock(func() error {
					p.updateCallMetrics(key)
					return nil
				})

				latency := float64(span.EndTime()-span.StartTime()) / float64(time.Millisecond.Nanoseconds())
				index := sort.SearchFloat64s(p.latencyBounds, latency)
				p.withLock(func() error {
					p.updateLatencyMetrics(key, latency, index)
					return nil
				})
			}
		}
	}
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
	kmap := map[string]string{
		serviceNameKey: serviceName,
		operationKey:   span.Name(),
		spanKindKey:    span.Kind().String(),
		statusCodeKey:  span.Status().Code().String(),
	}
	spanAttr := span.Attributes()
	for _, d := range p.dimensions {
		// Set the default if configured, otherwise this metric will have
		// no value set for the dimension.
		if d.Default != nil {
			kmap[d.Name] = *d.Default
		}
		// Map the various attribute values to string.
		if attr, ok := spanAttr.Get(d.Name); ok {
			switch attr.Type() {
			case pdata.AttributeValueSTRING:
				kmap[d.Name] = attr.StringVal()
			case pdata.AttributeValueINT:
				kmap[d.Name] = strconv.FormatInt(attr.IntVal(), 10)
			case pdata.AttributeValueDOUBLE:
				kmap[d.Name] = strconv.FormatFloat(attr.DoubleVal(), 'f', -1, 64)
			case pdata.AttributeValueBOOL:
				kmap[d.Name] = strconv.FormatBool(attr.BoolVal())
			case pdata.AttributeValueNULL:
				kmap[d.Name] = ""
			default:
				p.logger.Warn("unsupported tag data type: " + attr.Type().String())
			}
		}
	}

	kjson, err := p.jsonSerder.Marshal(kmap)
	if err != nil {
		return "", err
	}
	return string(kjson), nil
}

func (p *processorImp) withRLock(f func() error) error {
	p.lock.RLock()
	err := f()
	p.lock.RUnlock()
	return err
}

func (p *processorImp) withLock(f func() error) error {
	p.lock.Lock()
	err := f()
	p.lock.Unlock()
	return err
}
