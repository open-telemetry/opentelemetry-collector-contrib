// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogmetricreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogmetricreceiver"
import (
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	metricsV2 "github.com/DataDog/agent-payload/v5/gogen"
	processv1 "github.com/DataDog/agent-payload/v5/process"
	metricsV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
)

type commonResourceAttributes struct {
	origin   string
	ApiKey   string
	mwSource string
	host     string
}

var (
	ErrNoMetricsInPayload = errors.New("no metrics in datadog payload")
)
var datadogMetricTypeStrToEnum map[string]int32 = map[string]int32{
	"count": datadogMetricTypeCount,
	"gauge": datadogMetricTypeGauge,
	"rate":  datadogMetricTypeRate,
}

func skipDatadogMetrics(metricName string, metricType int32) bool {
	if strings.HasPrefix(metricName, "datadog") {
		return true
	}

	if strings.HasPrefix(metricName, "n_o_i_n_d_e_x.datadog") {
		return true
	}

	if metricType != datadogMetricTypeRate &&
		metricType != datadogMetricTypeGauge &&
		metricType != datadogMetricTypeCount {
		return true
	}
	return false
}

func setMetricResourceAttributes(attributes pcommon.Map,
	cra commonResourceAttributes) {
	if cra.origin != "" {
		attributes.PutStr("mw.client_origin", cra.origin)
	}
	if cra.ApiKey != "" {
		attributes.PutStr("mw.account_key", cra.ApiKey)
	}
	if cra.mwSource != "" {
		attributes.PutStr("mw_source", cra.mwSource)
	}
	if cra.host != "" {
		attributes.PutStr("host.id", cra.host)
		attributes.PutStr("host.name", cra.host)
	}
}

func getOtlpExportReqFromDatadogV1Metrics(origin string, key string,
	ddReq metricsV1.MetricsPayload) (pmetricotlp.ExportRequest, error) {

	fmt.Println("v1", len(ddReq.GetSeries()))
	if len(ddReq.GetSeries()) == 0 {
		fmt.Println("no metrics in the payload", "origin", origin, "key", key)
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	// assumption is that host is same for all the metrics in a given request
	var metricHost string
	if ddReq.Series[0].HasHost() {
		metricHost = ddReq.Series[0].GetHost()
	}

	commonResourceAttributes := commonResourceAttributes{
		origin:   origin,
		ApiKey:   key,
		mwSource: "datadog",
		host:     metricHost,
	}
	setMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName("mw")
	instrumentationScope.SetVersion("v0.0.1")

	for _, s := range ddReq.GetSeries() {
		// ignore any metric that begins with "datadog" or
		// the metric type is not yet supported by Middleware
		if skipDatadogMetrics(s.GetMetric(), datadogMetricTypeStrToEnum[s.GetType()]) {
			continue
		}

		scopeMetric := scopeMetrics.Metrics().AppendEmpty()
		scopeMetric.SetName(s.GetMetric())

		metricAttributes := pcommon.NewMap()

		for _, tag := range s.GetTags() {
			// Datadog sends tag as string slice. Each member
			// of the slice is of the form "<key>:<value>"
			// e.g. "client_version:5.1.1"
			parts := strings.Split(tag, ":")
			if len(parts) != 2 {
				continue
			}

			metricAttributes.PutStr(parts[0], parts[1])
		}

		var dataPoints pmetric.NumberDataPointSlice
		// in case datadog metric is rate, we need to multiply
		// the value in the metric by multiplyFactor to get the sum
		// for otlp metrics.
		multiplyFactor := 1.0
		switch datadogMetricTypeStrToEnum[s.GetType()] {
		case datadogMetricTypeRate:
			multiplyFactor = float64(s.GetInterval())
			fallthrough
		case datadogMetricTypeCount:
			sum := scopeMetric.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			sum.SetIsMonotonic(false)
			dataPoints = sum.DataPoints()
		case datadogMetricTypeGauge:
			gauge := scopeMetric.SetEmptyGauge()
			dataPoints = gauge.DataPoints()
		default:
			fmt.Println("datadog metric not yet handled", "type", s.Metric)
			continue
		}

		for _, point := range s.GetPoints() {
			// Datadog payload stores timestamp as first member of Point array
			unixNano := float64(*point[0]) * math.Pow(10, 9)
			dp := dataPoints.AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(unixNano))
			// Datadog payload stores count value as second member of Point
			// array
			dp.SetDoubleValue(float64(*point[1]) * multiplyFactor)
			attributeMap := dp.Attributes()
			metricAttributes.CopyTo(attributeMap)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

// handle V2 datadog metrics. The code is similar to getOtlpExportReqFromDatadogV1Metrics
// and feels repetitive, but structure of V1 and V2 payloads have different types for
// the same fields (e.g. Series.Points) and keeping these functions separate provide
// good readability
func getOtlpExportReqFromDatadogV2Metrics(origin string, key string,
	ddReq metricsV2.MetricPayload) (pmetricotlp.ExportRequest, error) {
	// assumption is that host is same for all the metrics in a given request

	if len(ddReq.GetSeries()) == 0 {
		fmt.Println("no metrics in the payload", "origin", origin, "key", key)
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	var metricHost string

	for _, series := range ddReq.GetSeries() {

		// Iterate through each resource in the series
		for i, resource := range series.GetResources() {
			if resource.GetType() == "host" {
				metricHost = resource.GetName()
			} else if i == 0 {
				resourceAttributes.PutStr(resource.GetType(), resource.GetName())
			}
		}
		// Break the outer loop if metricHost is set
		if metricHost != "" {
			break
		}
	}

	commonResourceAttributes := commonResourceAttributes{
		origin:   origin,
		ApiKey:   key,
		mwSource: "datadog",
		host:     metricHost,
	}
	setMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName("mw")
	instrumentationScope.SetVersion("v0.0.1")

	for _, s := range ddReq.GetSeries() {
		// ignore any metric that begins with "datadog" or
		// the metric type is not yet supported by Middleware
		if skipDatadogMetrics(s.GetMetric(), int32(s.GetType())) {
			continue
		}

		scopeMetric := scopeMetrics.Metrics().AppendEmpty()
		scopeMetric.SetName(s.GetMetric())
		scopeMetric.SetUnit(s.GetUnit())

		metricAttributes := pcommon.NewMap()

		for _, tag := range s.GetTags() {
			// Datadog v1 sends tag as string slice. Each member
			// of the slice is of the form "<key>:<value>"
			// e.g. "client_version:5.1.1"
			parts := strings.Split(tag, ":")
			if len(parts) != 2 {
				continue
			}

			metricAttributes.PutStr(parts[0], parts[1])
		}

		var dataPoints pmetric.NumberDataPointSlice
		// in case datadog metric is rate, we need to multiply
		// the value in the metric by multiplyFactor to get the sum
		// for otlp metrics.
		multiplyFactor := 1.0
		switch s.GetType() {
		case metricsV2.MetricPayload_RATE:
			multiplyFactor = float64(s.GetInterval())
			fallthrough
		case metricsV2.MetricPayload_COUNT:
			sum := scopeMetric.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			sum.SetIsMonotonic(false)
			dataPoints = sum.DataPoints()
		case metricsV2.MetricPayload_GAUGE:
			gauge := scopeMetric.SetEmptyGauge()
			dataPoints = gauge.DataPoints()
		default:
			fmt.Println("datadog metric not yet handled", "type", s.Metric)
			continue
		}

		for _, point := range s.GetPoints() {
			// Datadog payload stores timestamp as first member of Point array
			unixNano := float64(point.GetTimestamp()) * math.Pow(10, 9)
			dp := dataPoints.AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(unixNano))
			// Datadog payload stores count value as second member of Point
			// array
			dp.SetDoubleValue(float64(point.GetValue()) * multiplyFactor)
			attributeMap := dp.Attributes()
			metricAttributes.CopyTo(attributeMap)
		}
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func getOtlpExportReqFromDatadogV1MetaData(origin string, key string,
	ddReq MetaDataPayload) (pmetricotlp.ExportRequest, error) {
	// assumption is that host is same for all the metrics in a given request

	if ddReq.Metadata == nil {
		fmt.Println("no metadata found so skipping", "origin", origin, "key", key)
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	// assumption is that host is same for all the metrics in a given request
	var metricHost string
	metricHost = ddReq.Hostname

	commonResourceAttributes := commonResourceAttributes{
		origin:   origin,
		ApiKey:   key,
		mwSource: "datadog",
		host:     metricHost,
	}
	setMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName("mw")
	instrumentationScope.SetVersion("v0.0.1")

	scopeMetric := scopeMetrics.Metrics().AppendEmpty()
	scopeMetric.SetName("system.host.metadata")
	metricAttributes := pcommon.NewMap()
	metaData := ddReq.Metadata

	v2 := reflect.ValueOf(*metaData)
	for i := 0; i < v2.NumField(); i++ {
		field := v2.Field(i)
		fieldType := v2.Type().Field(i)
		val := fmt.Sprintf("%v", field.Interface())
		metricAttributes.PutStr(fieldType.Name, val)
		//fmt.Printf("Field Name: %s, Field Value: %s\n", fieldType.Name, val)
	}

	var dataPoints pmetric.NumberDataPointSlice
	gauge := scopeMetric.SetEmptyGauge()
	dataPoints = gauge.DataPoints()

	epoch := ddReq.Timestamp

	dp := dataPoints.AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(epoch))

	dp.SetDoubleValue(float64(10.54) * 1.0)
	attributeMap := dp.Attributes()
	metricAttributes.CopyTo(attributeMap)

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func getOtlpExportReqFromDatadogProcessesData(origin string, key string,
	ddReq *processv1.CollectorProc) (pmetricotlp.ExportRequest, error) {
	// assumption is that host is same for all the metrics in a given request

	if ddReq == nil {
		fmt.Println("no metadata found so skipping", "origin", origin, "key", key)
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	processPayload := ddReq.GetProcesses()

	if processPayload == nil || len(processPayload) == 0 {
		fmt.Println("no metadata found so skipping", "origin", origin, "key", key)
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	// assumption is that host is same for all the metrics in a given request
	var metricHost string
	metricHost = ddReq.HostName
	// processPayloadCopy := make([]*processv1.Process, len(processPayload))
	// copy(processPayloadCopy, processPayload)

	commonResourceAttributes := commonResourceAttributes{
		origin:   origin,
		ApiKey:   key,
		mwSource: "datadog",
		host:     metricHost,
	}
	setMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName("mw")
	instrumentationScope.SetVersion("v0.0.1")

	metrics_to_extract := []string{"system.process.rss", "system.process.total_cpu_pct"}

	for _, processs := range processPayload {

		if processs == nil {
			continue
		}

		for _, new_metric := range metrics_to_extract {

			var metric_val float64
			if new_metric == "system.process.rss" {
				memory_process := processs.GetMemory()
				if memory_process == nil {
					continue
				}
				metric_val = float64(memory_process.Rss)
				//mwlogger.Info("Memory...", metric_val)
			}

			if new_metric == "system.process.total_cpu_pct" {
				cpustat := processs.GetCpu()
				if cpustat == nil {
					continue
				}
				metric_val = float64(cpustat.TotalPct)
				//mwlogger.Info("System percentage", metric_val)
			}

			scopeMetric := scopeMetrics.Metrics().AppendEmpty()
			scopeMetric.SetName(new_metric)
			//scopeMetric.SetUnit(s.GetUnit())

			metricAttributes := pcommon.NewMap()

			// PROCESS ARGS
			command := processs.GetCommand()
			if command != nil {
				val := command.Args
				result := strings.Join(val, " ")
				metricAttributes.PutStr("process_name", result)
				// _ = process.Command.Ppid
				//mwlogger.Info("Cnmd", result)
				// mwlogger.Info("Ppid", process.Command.Ppid)
			}

			// GET USER INFO
			userinfo := processs.GetUser()
			if userinfo != nil {
				val := userinfo.Name
				//mwlogger.Info("Username", val)
				metricAttributes.PutStr("USERNAME", val)
			}

			// CREATETIME
			currentTime := time.Now()
			milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000
			createtime := (int64(milliseconds/1000000) - processs.CreateTime) / 1000
			//mwlogger.Info("Createtime", createtime)
			metricAttributes.PutInt("Createtime", createtime)

			var dataPoints pmetric.NumberDataPointSlice
			gauge := scopeMetric.SetEmptyGauge()
			dataPoints = gauge.DataPoints()

			dp := dataPoints.AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(milliseconds))
			// Datadog payload stores count value as second member of Point
			// array
			dp.SetDoubleValue(float64(metric_val) * 1.0)
			attributeMap := dp.Attributes()
			metricAttributes.CopyTo(attributeMap)
		}

	}
	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}

func convertSize(sizeInKB float64) string {
	units := []string{"K", "M", "G"}
	unitIndex := 0

	size := sizeInKB
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.2f%s", size, units[unitIndex])
}

func getOtlpExportReqFromDatadogIntakeData(origin string, key string,
	ddReq GoHaiData, input struct {
		hostname      string
		containerInfo map[string]string
		milliseconds  int64
	}) (pmetricotlp.ExportRequest, error) {
	// assumption is that host is same for all the metrics in a given request

	if len(ddReq.FileSystem) == 0 {
		log.Println("no metadata found so skipping")
		return pmetricotlp.ExportRequest{}, ErrNoMetricsInPayload
	}

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	// assumption is that host is same for all the metrics in a given request
	var metricHost string
	metricHost = input.hostname

	commonResourceAttributes := commonResourceAttributes{
		origin:   origin,
		ApiKey:   key,
		mwSource: "datadog",
		host:     metricHost,
	}
	setMetricResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeMetrics := rm.ScopeMetrics().AppendEmpty()
	instrumentationScope := scopeMetrics.Scope()
	instrumentationScope.SetName("mw")
	instrumentationScope.SetVersion("v0.0.1")

	for _, fileData := range ddReq.FileSystem {

		scopeMetric := scopeMetrics.Metrics().AppendEmpty()
		scopeMetric.SetName("system.intake.metadata")
		//scopeMetric.SetUnit(s.GetUnit())

		floatVal, err := strconv.ParseFloat(fileData.KbSize, 64)
		if err != nil {
			log.Println("error converting string to float64")
			return pmetricotlp.ExportRequest{}, err
		}

		metricAttributes := pcommon.NewMap()
		str := fileData.Name + " mounted on " + fileData.MountedOn + " " + convertSize(floatVal)
		metricAttributes.PutStr("FILESYSTEM", str)

		if docker_swarm, ok := input.containerInfo["docker_swarm"]; ok {
			metricAttributes.PutStr("docker_swarm", docker_swarm)
		}

		if docker_version, ok := input.containerInfo["docker_version"]; ok {
			metricAttributes.PutStr("docker_version", docker_version)
		}

		if kubelet_version, ok := input.containerInfo["kubelet_version"]; ok {
			metricAttributes.PutStr("kubelet_version", kubelet_version)
		}

		// current time in millis
		// currentTime := time.Now()
		// milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000

		var dataPoints pmetric.NumberDataPointSlice
		gauge := scopeMetric.SetEmptyGauge()
		dataPoints = gauge.DataPoints()

		dp := dataPoints.AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(input.milliseconds))

		dp.SetDoubleValue(1.0) // setting a dummy value for this metric as only resource attribute needed
		attributeMap := dp.Attributes()
		metricAttributes.CopyTo(attributeMap)
	}

	return pmetricotlp.NewExportRequestFromMetrics(metrics), nil
}
