package splunkexporter

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type splunkExporter struct {
	pushMetricsData func(ctx context.Context, md consumerdata.MetricsData) (droppedTimeSeries int, err error)
}

type exporterOptions struct {
  url   *url.URL
  token string
}


// New returns a new Splunk exporter.
func New(
	config *Config,
	logger *zap.Logger,
) (component.MetricsExporterOld, error) {

	if config == nil {
		return nil, errors.New("nil config")
	}

	options, err := config.getOptionsFromConfig()
	if err != nil {
		return nil,
			fmt.Errorf("failed to process %q config: %v", config.Name(), err)
	}

	logger.Info("Splunk Config", zap.String("url", options.url.String()))

	if config.Name() == "" {
		config.SetType(typeStr)
		config.SetName(typeStr)
	}

	dpClient := &client{
		url: options.url,
		client: &http.Client{
			// TODO: What other settings of http.Client to expose via config?
			//  Or what others change from default values?
			Timeout: 5 * time.Second,
		},
		logger: logger,
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
	}

	return splunkExporter{
		pushMetricsData: dpClient.pushMetricsData,
	}, nil
}

func (se splunkExporter) Start(context.Context, component.Host) error {
	return nil
}

func (se splunkExporter) Shutdown(context.Context) error {
	return nil
}

func (se splunkExporter) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	ctx = obsreport.StartMetricsExportOp(ctx, typeStr)
	numDroppedTimeSeries, err := se.pushMetricsData(ctx, md)

	numReceivedTimeSeries, numPoints := pdatautil.TimeseriesAndPointCount(md)

	obsreport.EndMetricsExportOp(ctx, numPoints, numReceivedTimeSeries, numDroppedTimeSeries, err)
	return err
}
