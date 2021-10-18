// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudspannerreceiver

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

//go:embed "internal/metadataconfig/metadata.yaml"
var metadataYaml []byte

var _ component.MetricsReceiver = (*googleCloudSpannerReceiver)(nil)

type googleCloudSpannerReceiver struct {
	logger         *zap.Logger
	nextConsumer   consumer.Metrics
	config         *Config
	cancel         context.CancelFunc
	projectReaders []statsreader.CompositeReader
	onCollectData  []func(error)
}

func newGoogleCloudSpannerReceiver(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {

	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	r := &googleCloudSpannerReceiver{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

func (r *googleCloudSpannerReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	err := r.initializeProjectReaders(ctx)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(r.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Ignoring this error because it has been already logged inside collectData
				r.notifyOnCollectData(r.collectData(ctx))
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (r *googleCloudSpannerReceiver) Shutdown(context.Context) error {
	for _, projectReader := range r.projectReaders {
		projectReader.Shutdown()
	}

	r.cancel()

	return nil
}

func (r *googleCloudSpannerReceiver) initializeProjectReaders(ctx context.Context) error {
	readerConfig := statsreader.ReaderConfig{
		BackfillEnabled:        r.config.BackfillEnabled,
		TopMetricsQueryMaxRows: r.config.TopMetricsQueryMaxRows,
	}

	parseMetadata, err := metadataparser.ParseMetadataConfig(metadataYaml)
	if err != nil {
		return fmt.Errorf("error occurred during parsing of metadata: %w", err)
	}

	for _, project := range r.config.Projects {
		projectReader, err := newProjectReader(ctx, r.logger, project, parseMetadata, readerConfig)
		if err != nil {
			return err
		}

		r.projectReaders = append(r.projectReaders, projectReader)
	}

	return nil
}

func newProjectReader(ctx context.Context, logger *zap.Logger, project Project, parsedMetadata []*metadata.MetricsMetadata,
	readerConfig statsreader.ReaderConfig) (*statsreader.ProjectReader, error) {
	logger.Debug("Constructing project reader for project", zap.String("project id", project.ID))

	databaseReadersCount := 0
	for _, instance := range project.Instances {
		databaseReadersCount += len(instance.Databases)
	}

	databaseReaders := make([]statsreader.CompositeReader, databaseReadersCount)
	databaseReaderIndex := 0
	for _, instance := range project.Instances {
		for _, database := range instance.Databases {
			logger.Debug("Constructing database reader for combination of project, instance, database",
				zap.String("project id", project.ID), zap.String("instance id", instance.ID), zap.String("database", database))

			databaseID := datasource.NewDatabaseID(project.ID, instance.ID, database)

			databaseReader, err := statsreader.NewDatabaseReader(ctx, parsedMetadata, databaseID,
				project.ServiceAccountKey, readerConfig, logger)
			if err != nil {
				return nil, err
			}

			databaseReaders[databaseReaderIndex] = databaseReader
			databaseReaderIndex++
		}
	}

	return statsreader.NewProjectReader(databaseReaders, logger), nil
}

func (r *googleCloudSpannerReceiver) collectData(ctx context.Context) error {
	var allMetrics []pdata.Metrics

	for _, projectReader := range r.projectReaders {
		allMetrics = append(allMetrics, projectReader.Read(ctx)...)
	}

	for _, metric := range allMetrics {
		if err := r.nextConsumer.ConsumeMetrics(ctx, metric); err != nil {
			// TODO Use obsreport instead to make the component observable and emit metrics on errors
			//r.logger.Error("Failed to consume metric(s) because of an error",
			//	zap.String("metric name", metricName(metric)), zap.Error(err))

			return err
		}
	}

	return nil
}

func (r *googleCloudSpannerReceiver) notifyOnCollectData(err error) {
	for _, onCollectData := range r.onCollectData {
		onCollectData(err)
	}
}

func metricName(metric pdata.Metrics) string {
	var mName string
	resourceMetrics := metric.ResourceMetrics()

	for i := 0; i < resourceMetrics.Len(); i++ {
		ilm := resourceMetrics.At(i).InstrumentationLibraryMetrics()

		for j := 0; j < ilm.Len(); j++ {
			metrics := ilm.At(j).Metrics()

			for k := 0; k < metrics.Len(); k++ {
				mName += metrics.At(k).Name() + ","
			}
		}
	}

	if mName != "" {
		mName = mName[:len(mName)-1]
	}

	return mName
}
