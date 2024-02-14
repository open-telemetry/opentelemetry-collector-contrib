// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"context"
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filterfactory"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

//go:embed "internal/metadataconfig/metrics.yaml"
var metadataYaml []byte

var _ receiver.Metrics = (*googleCloudSpannerReceiver)(nil)

type googleCloudSpannerReceiver struct {
	logger         *zap.Logger
	config         *Config
	cancel         context.CancelFunc
	projectReaders []statsreader.CompositeReader
	metricsBuilder metadata.MetricsBuilder
}

func newGoogleCloudSpannerReceiver(logger *zap.Logger, config *Config) *googleCloudSpannerReceiver {
	return &googleCloudSpannerReceiver{
		logger: logger,
		config: config,
	}
}

func (r *googleCloudSpannerReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	var (
		allMetricsDataPoints []*metadata.MetricsDataPoint
		err                  error
	)

	for _, projectReader := range r.projectReaders {
		dataPoints, readErr := projectReader.Read(ctx)
		allMetricsDataPoints = append(allMetricsDataPoints, dataPoints...)
		if readErr != nil {
			err = multierr.Append(err, readErr)
		}
	}

	metrics, buildErr := r.metricsBuilder.Build(allMetricsDataPoints)
	if buildErr != nil {
		err = multierr.Append(err, buildErr)
	}

	if err != nil && metrics.DataPointCount() > 0 {
		err = scrapererror.NewPartialScrapeError(err, len(multierr.Errors(err)))
	}
	return metrics, err
}

func (r *googleCloudSpannerReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	err := r.initialize(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *googleCloudSpannerReceiver) Shutdown(context.Context) error {
	for _, projectReader := range r.projectReaders {
		projectReader.Shutdown()
	}

	if r.metricsBuilder == nil {
		return nil
	}
	err := r.metricsBuilder.Shutdown()
	if err != nil {
		return err
	}

	r.cancel()

	return nil
}

func (r *googleCloudSpannerReceiver) initialize(ctx context.Context) error {
	parsedMetadata, err := metadataparser.ParseMetadataConfig(metadataYaml)
	if err != nil {
		return fmt.Errorf("error occurred during parsing of metadata: %w", err)
	}

	err = r.initializeProjectReaders(ctx, parsedMetadata)
	if err != nil {
		return err
	}

	return r.initializeMetricsBuilder(parsedMetadata)
}

func (r *googleCloudSpannerReceiver) initializeProjectReaders(ctx context.Context,
	parsedMetadata []*metadata.MetricsMetadata) error {

	readerConfig := statsreader.ReaderConfig{
		BackfillEnabled:                   r.config.BackfillEnabled,
		TopMetricsQueryMaxRows:            r.config.TopMetricsQueryMaxRows,
		HideTopnLockstatsRowrangestartkey: r.config.HideTopnLockstatsRowrangestartkey,
		TruncateText:                      r.config.TruncateText,
	}

	for _, project := range r.config.Projects {
		projectReader, err := newProjectReader(ctx, r.logger, project, parsedMetadata, readerConfig)
		if err != nil {
			return err
		}

		r.projectReaders = append(r.projectReaders, projectReader)
	}

	return nil
}

func (r *googleCloudSpannerReceiver) initializeMetricsBuilder(parsedMetadata []*metadata.MetricsMetadata) error {
	r.logger.Debug("Constructing metrics builder")

	projectAmount := len(r.config.Projects)
	instanceAmount := 0
	databaseAmount := 0

	for _, project := range r.config.Projects {
		instanceAmount += len(project.Instances)

		for _, instance := range project.Instances {
			databaseAmount += len(instance.Databases)
		}
	}

	factoryConfig := &filterfactory.ItemFilterFactoryConfig{
		MetadataItems:  parsedMetadata,
		TotalLimit:     r.config.CardinalityTotalLimit,
		ProjectAmount:  projectAmount,
		InstanceAmount: instanceAmount,
		DatabaseAmount: databaseAmount,
	}
	itemFilterResolver, err := filterfactory.NewItemFilterResolver(r.logger, factoryConfig)
	if err != nil {
		return err
	}

	r.metricsBuilder = metadata.NewMetricsFromDataPointBuilder(itemFilterResolver)

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
