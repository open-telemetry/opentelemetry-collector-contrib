package awsemfexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/translator"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	defaultForceFlushInterval = 60 * time.Second
	defaultForceFlushIntervalInSeconds = 60
)

type emfExporter struct {

	//Each (log group, log stream) keeps a separate Pusher because of each (log group, log stream) requires separate stream token.
	groupStreamToPusherMap map[string]map[string]Pusher
	svcStructuredLog LogClient
	config configmodels.Exporter
	logger *zap.Logger

	pusherMapLock sync.Mutex
	pusherWG         sync.WaitGroup
	ForceFlushInterval time.Duration
	shutdownChan chan bool
	retryCnt int

}

// New func creates an EMF Exporter instance with data push callback func
func New(
	config configmodels.Exporter,
	params component.ExporterCreateParams,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("emf exporter config is nil")
	}

	logger := params.Logger
	// create AWS session
	awsConfig, session, err := GetAWSConfigSession(logger, &Conn{}, config.(*Config))
	if err != nil {
		return nil, err
	}

	// create CWLogs client with aws session config
	svcStructuredLog := NewCloudWatchLogsClient(logger, awsConfig, session)

	emfExporter := &emfExporter{
		svcStructuredLog: svcStructuredLog,
		config: config,
		ForceFlushInterval: defaultForceFlushInterval,
		retryCnt: *awsConfig.MaxRetries,
		logger: logger,
	}
	if config.(*Config).ForceFlushInterval > 0 {
		emfExporter.ForceFlushInterval = time.Duration(config.(*Config).ForceFlushInterval) * time.Second
	}
	emfExporter.groupStreamToPusherMap = map[string]map[string]Pusher{}
	emfExporter.shutdownChan = make(chan bool)

	return emfExporter, nil
}

func (emf *emfExporter) pushMetricsData(_ context.Context, md pdata.Metrics) (droppedTimeSeries int, err error) {
	expConfig := emf.config.(*Config)
	logGroup := "/metrics/default"
	logStream := "otel-stream"
	// override log group if customer has specified Resource Attributes service.name or service.namespace
	putLogEvents, namespace := generateLogEventFromMetric(md)
	if namespace != "" {
		logGroup = fmt.Sprintf("/metrics/%s", namespace)
	}
	// override log group if found it in exp configuration, this configuration has top priority. However, in this case, customer won't have correlation experience
	if len(expConfig.LogGroupName) > 0 {
		logGroup = expConfig.LogGroupName
	}
	if len(expConfig.LogStreamName) > 0 {
		logStream = expConfig.LogStreamName
	}
	pusher := emf.getPusher(logGroup, logStream, nil)
	if pusher != nil {
		for _, ple := range putLogEvents {
			pusher.AddLogEntry(ple)
		}
	}
	return 0, nil
}

func (emf *emfExporter) getPusher(logGroup, logStream string, stateFolder *string) Pusher {
	emf.pusherMapLock.Lock()
	defer emf.pusherMapLock.Unlock()

	var ok bool
	var streamToPusherMap map[string]Pusher
	if streamToPusherMap, ok = emf.groupStreamToPusherMap[logGroup]; !ok {
		streamToPusherMap = map[string]Pusher{}
		emf.groupStreamToPusherMap[logGroup] = streamToPusherMap
	}

	var pusher Pusher
	if pusher, ok = streamToPusherMap[logStream]; !ok {
		pusher = NewPusher(
			aws.String(logGroup), aws.String(logStream), stateFolder,
			emf.ForceFlushInterval, emf.retryCnt, emf.svcStructuredLog, emf.shutdownChan, &emf.pusherWG)
		streamToPusherMap[logStream] = pusher
	}

	return pusher
}

func (emf *emfExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	exporterCtx := obsreport.ExporterContext(ctx, "emf.exporterFullName")

	_, err := emf.pushMetricsData(exporterCtx, md)
	return err
}

// Shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) Shutdown(ctx context.Context) error {
	close(emf.shutdownChan)
	emf.pusherWG.Wait()
	return nil
}

// Start
func (emf *emfExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}


func generateLogEventFromMetric(metric pdata.Metrics) ([]*LogEvent, string) {
	imd := pdatautil.MetricsToInternalMetrics(metric)
	rms := imd.ResourceMetrics()
	cwMetricLists := []*translator.CWMetrics{}
	var namespace string
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if rm.IsNil() {
			continue
		}
		// translate OT metric datapoints into CWMetricLists
		cwm, err := translator.TranslateOtToCWMetric(&rm)
		if err != nil || cwm == nil {
			return nil, ""
		}
		if len(cwm) > 0 && len(cwm[0].Measurements) > 0 {
			namespace = cwm[0].Measurements[0].Namespace
		}
		// append all datapoint metrics in the request into CWMetric list
		for _, v := range cwm {
			cwMetricLists = append(cwMetricLists, v)
		}
	}

	// convert CWMetric into map format for compatible with PLE input
	ples := make([]*LogEvent, 0, maximumLogEventsPerPut)
	for _, met := range cwMetricLists {
		cwmMap := make(map[string]interface{})
		fieldMap := met.Fields
		cwmMap["CloudWatchMetrics"] = met.Measurements
		cwmMap["Timestamp"] = met.Timestamp
		fieldMap["_aws"] = cwmMap

		pleMsg, err := json.Marshal(fieldMap)
		if err != nil {
			continue
		}
		metricCreationTime := met.Timestamp

		logEvent := NewLogEvent(
			metricCreationTime,
			string(pleMsg),
			"",
			0,
			Structured,
		)
		logEvent.multiLineStart = true
		logEvent.LogGeneratedTime = time.Unix(0, metricCreationTime * int64(time.Millisecond))
		ples = append(ples, logEvent)
	}
	return ples, namespace
}