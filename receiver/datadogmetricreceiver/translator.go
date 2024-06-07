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

var metrics_to_extract = map[string]map[string]string{
	"system.process.voluntary_context_switches": {
		"field": "VoluntaryCtxSwitches",
		"type":  "",
	},
	"system.process.involuntary_context_switches": {
		"field": "InvoluntaryCtxSwitches",
		"type":  "",
	},
	"system.process.open_file_descriptors": {
		"field": "OpenFdCount",
		"type":  "",
	},
	"system.process.create_time": {
		"type":  "",
		"field": "CreateTime",
	},
	"system.process.cpu.total_percentage": {
		"type":  "cpu",
		"field": "TotalPct",
	},
	"system.process.cpu.user_percentage": {
		"type":  "cpu",
		"field": "UserPct",
	},
	"system.process.cpu.system_percentage": {
		"type":  "cpu",
		"field": "SystemPct",
	},
	"system.process.threads_count": {
		"type":  "cpu",
		"field": "NumThreads",
	},
	"system.process.rss": {
		"type":  "memory",
		"field": "Rss",
	},
	"system.process.vms": {
		"type":  "memory",
		"field": "Vms",
	},
}

var container_metrics_to_extract = map[string]string{
	//"container.process.status":                "State",
	"container.process.create_time":           "CreateTime",
	"container.process.cpu.total_percentage":  "TotalPct",
	"container.process.cpu.user_percentage":   "UserPct",
	"container.process.cpu.system_percentage": "SystemPct",
	"container.process.net_bytes_sent":        "NetSentBps",
	"container.process.net_bytes_rcvd":        "NetRcvdBps",
	"container.process.rss":                   "MemRss",
	"container.process.ioread":                "Rbps",
	"container.process.iowrite":               "Wbps",
	"container.process.start_time":            "StartTime",
}

var ContainerState_name = map[int32]string{
	0: "unknown",
	1: "created",
	2: "restarting",
	3: "running",
	4: "paused",
	5: "exited",
	6: "dead",
}

var ContainerHealth_name = map[int32]string{
	0: "unknownHealth",
	1: "starting",
	2: "healthy",
	3: "unhealthy",
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
		metricAttributes.PutBool("datadog_metric" , true)
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
		metricAttributes.PutBool("datadog_metric" , true)
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
	metricAttributes.PutBool("datadog_metric" , true)
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

	for _, processs := range processPayload {

		if processs == nil {
			continue
		}

		for new_metric, metric_map := range metrics_to_extract {

			var metric_val float64
			if metric_map["type"] == "memory" {
				memory_process := processs.GetMemory()
				if memory_process == nil {
					continue
				}

				switch metric_map["field"] {
				case "Rss":
					metric_val = float64(memory_process.GetRss())
				case "Vms":
					metric_val = float64(memory_process.GetVms())
				default:
					continue
				}
			}

			if metric_map["type"] == "cpu" {
				cpustat := processs.GetCpu()
				if cpustat == nil {
					continue
				}
				switch metric_map["field"] {
				case "TotalPct":
					metric_val = float64(cpustat.GetTotalPct())
				case "UserPct":
					metric_val = float64(cpustat.GetUserPct())
				case "SystemPct":
					metric_val = float64(cpustat.GetSystemPct())
				case "NumThreads":
					metric_val = float64(cpustat.GetNumThreads())
				default:
					continue
				}
			}

			if metric_map["type"] == "" {
				switch metric_map["field"] {
				case "VoluntaryCtxSwitches":
					metric_val = float64(processs.GetVoluntaryCtxSwitches())
				case "InvoluntaryCtxSwitches":
					metric_val = float64(processs.GetInvoluntaryCtxSwitches())
				case "OpenFdCount":
					metric_val = float64(processs.GetOpenFdCount())
				case "CreateTime":
					currentTime := time.Now()
					milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000
					createtime := (int64(milliseconds/1000000) - processs.GetCreateTime()) / 1000
					metric_val = float64(createtime)
				default:
					continue
				}
			}

			scopeMetric := scopeMetrics.Metrics().AppendEmpty()
			scopeMetric.SetName(new_metric)
			//scopeMetric.SetUnit(s.GetUnit())

			metricAttributes := pcommon.NewMap()
			metricAttributes.PutBool("datadog_metric" , true)
			// PROCESS ARGS
			command := processs.GetCommand()
			if command != nil {
				val := command.Args
				result := strings.Join(val, " ")
				metricAttributes.PutStr("process_name", result)
			}

			// GET USER INFO
			userinfo := processs.GetUser()
			if userinfo != nil {
				val := userinfo.Name
				metricAttributes.PutStr("USERNAME", val)
			}

			// GET PID
			pid := processs.GetPid()
			metricAttributes.PutInt("pid", int64(pid))

			// CREATETIME
			currentTime := time.Now()
			milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000
			
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

	containerPayload := ddReq.GetContainers()

	for _, container := range containerPayload {

		if container == nil {
			continue
		}

		for new_metric, field := range container_metrics_to_extract {

			var metric_val float64
			currentTime := time.Now()
			milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000

			switch field {
			case "CreateTime":
				// Handle CreateTime metric
				createtime := (int64(milliseconds/1000000000) - container.GetCreated())
				metric_val = float64(createtime)
			case "TotalPct":
				// Handle TotalPct metric
				metric_val = float64(container.GetTotalPct())
			case "UserPct":
				// Handle UserPct metric
				metric_val = float64(container.GetUserPct())
			case "SystemPct":
				// Handle SystemPct metric
				metric_val = float64(container.GetSystemPct())
			case "NetSentBps":
				// Handle NetSentBps metric
				metric_val = float64(container.GetNetSentBps())
			case "NetRcvdBps":
				// Handle NetRcvdBps metric
				metric_val = float64(container.GetNetRcvdBps())
			case "MemRss":
				// Handle MemRss metric
				metric_val = float64(container.GetMemRss())
			case "Rbps":
				// Handle Rbps metric
				metric_val = float64(container.GetRbps())
			case "Wbps":
				// Handle Wbps metric
				metric_val = float64(container.GetWbps())
			case "StartTime":
				starttime := (int64(milliseconds/1000000000) - container.GetStarted())
				metric_val = float64(starttime)
			default:
				fmt.Printf("Unknown field: %s\n", field)
			}

			scopeMetric := scopeMetrics.Metrics().AppendEmpty()
			scopeMetric.SetName(new_metric)
			//scopeMetric.SetUnit(s.GetUnit())

			metricAttributes := pcommon.NewMap()
			metricAttributes.PutBool("datadog_metric" , true)
			metricAttributes.PutStr("container_id", container.GetId())
			metricAttributes.PutStr("container_name", container.GetName())
			metricAttributes.PutStr("container_image", container.GetImage())
			metricAttributes.PutStr("container_status", ContainerState_name[int32(container.GetState())])
			metricAttributes.PutStr("container_health", ContainerHealth_name[int32(container.GetHealth())])

			tags := container.GetTags()
			if tags != nil && len(tags) > 0 {
				metricAttributes.PutStr("container_tags", strings.Join(tags, "&"))
			}

			for _, tag := range tags {
				// Datadog sends tag as string slice. Each member
				// of the slice is of the form "<key>:<value>"
				// e.g. "client_version:5.1.1"
				parts := strings.Split(tag, ":")
				if len(parts) != 2 {
					continue
				}

				metricAttributes.PutStr(parts[0], parts[1])
			}

			// CREATETIME
			// currentTime := time.Now()
			// milliseconds := (currentTime.UnixNano() / int64(time.Millisecond)) * 1000000

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
		metricAttributes.PutBool("datadog_metric" , true)
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
