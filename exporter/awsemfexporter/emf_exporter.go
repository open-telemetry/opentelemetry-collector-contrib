package awsemfexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
)

type emfExporter struct {
	pushMetricsData func(ctx context.Context, md pdata.Metrics) (droppedTimeSeries int, err error)
}

// New returns test emf
func New(
	config configmodels.Exporter,
	params component.ExporterCreateParams,
) (component.MetricsExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	logger := params.Logger
	awsConfig, session, err := GetAWSConfigSession(logger, &Conn{}, config.(*Config))
	if err != nil {
		return nil, err
	}
	client := NewCloudWatchLogsClient(logger, awsConfig, session)
	cwlClient := &cwlClient{
		logger: logger,
		client: client,
		config: config,
	}

	return &emfExporter{
		pushMetricsData: cwlClient.pushMetricsData,
	}, nil
}

func (emf *emfExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	exporterCtx := obsreport.ExporterContext(ctx, "emf.exporterFullName")

	_, err := emf.pushMetricsData(exporterCtx, md)
	return err
}

// Shutdown stops the exporter and is invoked during shutdown.
func (emf *emfExporter) Shutdown(ctx context.Context) error {
	return nil
}

// Start
func (emf *emfExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}
