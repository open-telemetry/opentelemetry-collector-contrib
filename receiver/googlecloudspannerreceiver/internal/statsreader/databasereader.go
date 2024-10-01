// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"

import (
	"context"
	"fmt"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type DatabaseReader struct {
	database *datasource.Database
	logger   *zap.Logger
	readers  []Reader
}

func NewDatabaseReader(ctx context.Context,
	parsedMetadata []*metadata.MetricsMetadata,
	databaseID *datasource.DatabaseID,
	serviceAccountPath string,
	readerConfig ReaderConfig,
	logger *zap.Logger) (*DatabaseReader, error) {

	database, err := datasource.NewDatabase(ctx, databaseID, serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("error occurred during client instantiation for database %q: %w", databaseID.ID(), err)
	}

	readers := initializeReaders(logger, parsedMetadata, database, readerConfig)

	return &DatabaseReader{
		database: database,
		logger:   logger,
		readers:  readers,
	}, nil
}

func initializeReaders(logger *zap.Logger, parsedMetadata []*metadata.MetricsMetadata,
	database *datasource.Database, readerConfig ReaderConfig) []Reader {
	readers := make([]Reader, len(parsedMetadata))

	for i, mData := range parsedMetadata {
		switch mData.MetadataType() {
		case metadata.MetricsMetadataTypeCurrentStats:
			readers[i] = newCurrentStatsReader(logger, database, mData, readerConfig)
		case metadata.MetricsMetadataTypeIntervalStats:
			readers[i] = newIntervalStatsReader(logger, database, mData, readerConfig)
		}
	}

	return readers
}

func (databaseReader *DatabaseReader) Name() string {
	return databaseReader.database.DatabaseID().ID()
}

func (databaseReader *DatabaseReader) Shutdown() {
	databaseReader.logger.Debug(
		"Closing connection to database",
		zap.String("database", databaseReader.database.DatabaseID().ID()),
	)
	databaseReader.database.Client().Close()
}

func (databaseReader *DatabaseReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	databaseReader.logger.Debug(
		"Executing read method for database",
		zap.String("database", databaseReader.database.DatabaseID().ID()),
	)

	var (
		result []*metadata.MetricsDataPoint
		err    error
	)

	for _, reader := range databaseReader.readers {
		dataPoints, readErr := reader.Read(ctx)
		result = append(result, dataPoints...)
		if readErr != nil {
			err = multierr.Append(
				err,
				fmt.Errorf("cannot read data for data points databaseReader %q because of an error: %w", reader.Name(), readErr),
			)
		}
	}
	if err != nil {
		databaseReader.logger.Warn(
			"Errors encountered while reading database",
			zap.String("database", databaseReader.database.DatabaseID().ID()),
			zap.Int("error_count", len(multierr.Errors(err))),
		)
	}

	return result, err
}
