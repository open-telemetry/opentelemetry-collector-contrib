package translator

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"go.opentelemetry.io/collector/translator/conventions"
	"sort"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/mapWithExpiry"
	"go.opentelemetry.io/collector/consumer/pdata"
)

const (
	CleanInterval = 5 * time.Minute
	MinTimeDiff   = 50 // We assume 50 micro-seconds is the minimal gap between two collected data sample to be valid to calculate delta

	//The following constants indicate the existence of resource attribute service.name and service.namespace
	ServiceNameOnly = iota
	ServiceNamespaceOnly
	ServiceNameAndNamespace
	ServiceNotDefined

	OtlibDimensionKey = "OTLib"
	defaultNameSpace = "default"
	noInstrumentationLibraryName = "Undefined"
)

var currentState = mapWithExpiry.NewMapWithExpiry(CleanInterval)


type rateState struct {
	value     interface{}
	timestamp int64
}

// CWMetrics defines
type CWMetrics struct {
	Measurements []CwMeasurement
	Timestamp    int64
	Fields       map[string]interface{}
}

// CwMeasurement defines
type CwMeasurement struct {
	Namespace  string
	Dimensions [][]string
	Metrics    []map[string]string
}

// CWMetric stats defines
type CWMetricStats struct {
	Max 		 float64
	Min 		 float64
	Count    	 uint64
	Sum			 float64
}

// TranslateOtToCWMetric converts OT metrics to CloudWatch Metric format
func TranslateOtToCWMetric(rm *pdata.ResourceMetrics) ([]*CWMetrics, error) {
	cwMetricLists := []*CWMetrics{}
	namespace := defaultNameSpace
	svcAttrMode := ServiceNotDefined

	if !rm.Resource().IsNil() {
		serviceName, svcNameOk := rm.Resource().Attributes().Get(conventions.AttributeServiceName)
		serviceNamespace, svcNsOk:= rm.Resource().Attributes().Get(conventions.AttributeServiceNamespace)

		if svcNameOk && serviceName.Type() == pdata.AttributeValueSTRING {
			svcAttrMode = ServiceNameOnly
		}
		if svcNsOk && serviceNamespace.Type() == pdata.AttributeValueSTRING {
			if ServiceNameOnly == svcAttrMode {
				svcAttrMode = ServiceNameAndNamespace
			} else {
				svcAttrMode = ServiceNamespaceOnly
			}
		}
		switch svcAttrMode {
		case ServiceNameAndNamespace:
			namespace = fmt.Sprintf("%s/%s", serviceNamespace.StringVal(), serviceName.StringVal())
		case ServiceNameOnly:
			namespace = fmt.Sprintf("%s", serviceName.StringVal())
		case ServiceNamespaceOnly:
			namespace = fmt.Sprintf("%s", serviceNamespace.StringVal())
		case ServiceNotDefined:
		default:
		}
	}
	ilms := rm.InstrumentationLibraryMetrics()
	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		if ilm.IsNil() {
			continue
		}
		if ilm.InstrumentationLibrary().IsNil() {
			ilm.InstrumentationLibrary().InitEmpty()
			ilm.InstrumentationLibrary().SetName(noInstrumentationLibraryName)
		}
		OTLib := ilm.InstrumentationLibrary().Name()
		metrics := ilm.Metrics()
		for k := 0; k < metrics.Len(); k++ {
			metric := metrics.At(k)
			if metric.IsNil() {
				continue
			}
			cwMetricList, err := getMeasurements(&metric, namespace, OTLib)
			if err != nil {
				continue
			}
			for _, v := range cwMetricList {
				cwMetricLists = append(cwMetricLists, v)
			}
		}
	}
	return cwMetricLists, nil
}

func getMeasurements(metric *pdata.Metric, namespace string, OTLib string) ([]*CWMetrics, error) {
	// Histogram data are not supported for now
	if metric.Int64DataPoints().Len() == 0 && metric.DoubleDataPoints().Len() == 0 && metric.SummaryDataPoints().Len() == 0 {
		return nil, nil
	}
	mDesc := metric.MetricDescriptor()
	if mDesc.IsNil() {
		return nil, nil
	}

	var result []*CWMetrics
	// metric measure data from OT
	metricMeasure := make(map[string]string)
	// metric measure slice could include multiple metric measures
	metricSlice := []map[string]string{}
	metricMeasure["Name"] = mDesc.Name()
	metricMeasure["Unit"] = mDesc.Unit()
	metricSlice = append(metricSlice, metricMeasure)

	// get all double datapoints
	ddp := metric.DoubleDataPoints()
	for m := 0; m < ddp.Len(); m++ {
		dp := ddp.At(m)
		if dp.IsNil() {
			continue
		}
		cwMetric := buildCWMetricFromDDP(dp, mDesc, namespace, metricSlice, OTLib)
		if cwMetric != nil {
			result = append(result, cwMetric)
		}
	}
	// get all int64 datapoints
	idp := metric.Int64DataPoints()
	for m := 0; m < idp.Len(); m++ {
		dp := idp.At(m)
		if dp.IsNil() {
			continue
		}
		cwMetric := buildCWMetricFromIDP(dp, mDesc, namespace, metricSlice, OTLib)
		if cwMetric != nil {
			result = append(result, cwMetric)
		}
	}
	// get all summary datapoints
	sdp := metric.SummaryDataPoints()
	for m := 0; m < sdp.Len(); m++ {
		dp := sdp.At(m)
		if dp.IsNil() {
			continue
		}
		cwMetric := buildCWMetricFromSDP(dp, mDesc, namespace, metricSlice, OTLib)
		if cwMetric != nil {
			result = append(result, cwMetric)
		}
	}
	return result, nil
}

func buildCWMetricFromDDP(metric pdata.DoubleDataPoint, mDesc pdata.MetricDescriptor, namespace string, metricSlice []map[string]string, OTLib string) *CWMetrics {

	// fields contains metric and dimensions key/value pairs
	fieldsPairs := make(map[string]interface{})
	// Dimensions Slice
	var dimensionSlice []string
	dimensionKV := metric.LabelsMap()
	dimensionKV.ForEach(func(k string, v pdata.StringValue) {
		fieldsPairs[k] = v.Value()
		dimensionSlice = append(dimensionSlice, k)
	})
	// add OTLib as an additional dimension
	fieldsPairs[OtlibDimensionKey] = OTLib
	dimensionSlice = append(dimensionSlice, OtlibDimensionKey)

	fieldsPairs[mDesc.Name()] = metric.Value()
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	metricVal := calculateRate(fieldsPairs, metric.Value(), timestamp)
	if metricVal == nil {
		return nil
	}
	fieldsPairs[mDesc.Name()] = metricVal

	// EMF dimension attr takes list of list on dimensions TODO: add single/zero dimension rollup here
	var dimensionArray [][]string
	dimensionArray = append(dimensionArray, dimensionSlice)
	cwMeasurement := &CwMeasurement{
		Namespace:  namespace,
		Dimensions: dimensionArray,
		Metrics:    metricSlice,
	}
	metricList := make([]CwMeasurement, 1)
	metricList[0] = *cwMeasurement
	cwMetric := &CWMetrics{
		Measurements: metricList,
		Timestamp:    timestamp,
		Fields:       fieldsPairs,
	}
	return cwMetric
}


func buildCWMetricFromIDP(metric pdata.Int64DataPoint, mDesc pdata.MetricDescriptor, namespace string, metricSlice []map[string]string, OTLib string) *CWMetrics {

	// fields contains metric and dimensions key/value pairs
	fieldsPairs := make(map[string]interface{})
	// Dimensions Array
	var dimensionArray [][]string
	var dimensionSlice []string
	dimensionKV := metric.LabelsMap()
	dimensionKV.ForEach(func(k string, v pdata.StringValue) {
		fieldsPairs[k] = v.Value()
		dimensionSlice = append(dimensionSlice, k)
	})
	// add OTLib as an additional dimension
	fieldsPairs[OtlibDimensionKey] = OTLib
	dimensionArray = append(dimensionArray, append(dimensionSlice, OtlibDimensionKey))

	fieldsPairs[mDesc.Name()] = metric.Value()
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	metricVal := calculateRate(fieldsPairs, metric.Value(), timestamp)
	if metricVal == nil {
		return nil
	}
	fieldsPairs[mDesc.Name()] = metricVal

	// EMF dimension attr takes list of list on dimensions. Including single/zero dimension rollup
	//"Zero" dimension rollup
	dimensionZero := []string{OtlibDimensionKey}
	if len(dimensionSlice) > 0 {
		dimensionArray = append(dimensionArray, dimensionZero)
	}
	//"One" dimension rollup
	for _, dimensionKey := range dimensionSlice {
		dimensionArray = append(dimensionArray, append(dimensionZero, dimensionKey))
	}

	cwMeasurement := &CwMeasurement{
		Namespace:  namespace,
		Dimensions: dimensionArray,
		Metrics:    metricSlice,
	}
	metricList := make([]CwMeasurement, 1)
	metricList[0] = *cwMeasurement
	cwMetric := &CWMetrics{
		Measurements: metricList,
		Timestamp:    timestamp,
		Fields:       fieldsPairs,
	}
	return cwMetric
}

func buildCWMetricFromSDP(metric pdata.SummaryDataPoint, mDesc pdata.MetricDescriptor, namespace string, metricSlice []map[string]string, OTLib string) *CWMetrics {

	// fields contains metric and dimensions key/value pairs
	fieldsPairs := make(map[string]interface{})
	// Dimensions Slice
	var dimensionSlice []string
	dimensionKV := metric.LabelsMap()
	dimensionKV.ForEach(func(k string, v pdata.StringValue) {
		fieldsPairs[k] = v.Value()
		dimensionSlice = append(dimensionSlice, k)
	})
	// add OTLib as an additional dimension
	fieldsPairs[OtlibDimensionKey] = OTLib
	dimensionSlice = append(dimensionSlice, OtlibDimensionKey)

	summaryValueAtPercentileSlice := metric.ValueAtPercentiles()
	metricStats := &CWMetricStats{
		Min: summaryValueAtPercentileSlice.At(0).Value(),
		Max: summaryValueAtPercentileSlice.At(summaryValueAtPercentileSlice.Len() - 1).Value(),
		Count: metric.Count(),
		Sum: metric.Sum(),
	}
	fieldsPairs[mDesc.Name()] = metricStats
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	// EMF dimension attr takes list of list on dimensions TODO: add single/zero dimension rollup here
	var dimensionArray [][]string
	dimensionArray = append(dimensionArray, dimensionSlice)
	cwMeasurement := &CwMeasurement{
		Namespace:  namespace,
		Dimensions: dimensionArray,
		Metrics:    metricSlice,
	}
	metricList := make([]CwMeasurement, 1)
	metricList[0] = *cwMeasurement
	cwMetric := &CWMetrics{
		Measurements: metricList,
		Timestamp:    timestamp,
		Fields:       fieldsPairs,
	}
	return cwMetric
}

// rate is calculated by valDelta / timeDelta
func calculateRate(fields map[string]interface{}, val interface{}, timestamp int64) interface{} {
	keys := make([]string, 0, len(fields))
	var b bytes.Buffer
	var metricRate interface{}
	// hash the key of str: metric + dimension key/value pairs (sorted alpha)
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch v := fields[k].(type) {
		case int64:
			b.WriteString(k)
			continue
		case string:
			b.WriteString(k)
			b.WriteString(v)
		default:
			continue
		}
	}
	h := sha1.New()
	h.Write(b.Bytes())
	bs := h.Sum(nil)
	hashStr := string(bs)

	// get previous Metric content from map
	if state, ok := currentState.Get(hashStr); ok {
		prevStats := state.(*rateState)
		deltaTime := timestamp - prevStats.timestamp
		var deltaVal interface{}
		if _, ok := val.(float64); ok {
			deltaVal = val.(float64) - prevStats.value.(float64)
			if deltaTime > MinTimeDiff && deltaVal.(float64) >= 0 {
				metricRate = deltaVal.(float64)*1e3 / float64(deltaTime)
			}
		} else {
			deltaVal = val.(int64) - prevStats.value.(int64)
			if deltaTime > MinTimeDiff && deltaVal.(int64) >= 0 {
				metricRate = deltaVal.(int64)*1e3 / deltaTime
			}
		}
	}
	content := &rateState{
		value:     val,
		timestamp: timestamp,
	}
	currentState.Set(hashStr, content)
	if metricRate == nil {
		metricRate = 0
	}
	return metricRate
}
