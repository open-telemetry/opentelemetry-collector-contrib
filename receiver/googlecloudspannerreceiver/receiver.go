// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sync"

	// Import the Spanner Admin Database API client and protobufs.
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	databasepb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
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
	readerConfig   statsreader.ReaderConfig
	parsedMetadata []*metadata.MetricsMetadata

	// mu protects access to the databaseReaders map in dynamic mode.
	mu sync.Mutex
	// databaseReaders holds the readers for active databases when in dynamic mode.
	databaseReaders map[string]statsreader.CompositeReader
}

func newGoogleCloudSpannerReceiver(logger *zap.Logger, config *Config) *googleCloudSpannerReceiver {
	return &googleCloudSpannerReceiver{
		logger:          logger,
		config:          config,
		databaseReaders: make(map[string]statsreader.CompositeReader),
	}
}

// Scrape acts as a dispatcher, choosing the scraping method based on the configuration.
func (r *googleCloudSpannerReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	dynamicMode := false
	for _, project := range r.config.Projects {
		for _, instance := range project.Instances {
			if instance.ScrapeAllDatabases {
				dynamicMode = true
				break
			}
		}
		if dynamicMode {
			break
		}
	}

	if dynamicMode {
		// Use the new dynamic discovery method.
		return r.scrapeDynamic(ctx)
	}

	// Use the original static method.
	return r.scrapeStatic(ctx)
}

// scrapeStatic contains the original scraping logic for statically configured databases.
func (r *googleCloudSpannerReceiver) scrapeStatic(ctx context.Context) (pmetric.Metrics, error) {
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

// scrapeDynamic handles scraping for environments with at least one dynamically configured instance.
func (r *googleCloudSpannerReceiver) scrapeDynamic(ctx context.Context) (pmetric.Metrics, error) {
	r.logger.Debug("Executing dynamic scrape.")
	r.mu.Lock()
	defer r.mu.Unlock()

	activeReaders := make(map[string]statsreader.CompositeReader)
	var allErrors error

	for _, project := range r.config.Projects {
		for _, instance := range project.Instances {
			databaseNames, err := r.getDatabaseNames(ctx, project, instance)
			if err != nil {
				allErrors = multierr.Append(allErrors, fmt.Errorf("failed to get database list for instance %s/%s: %w", project.ID, instance.ID, err))
				continue
			}

			for _, dbName := range databaseNames {
				readerKey := fmt.Sprintf("%s/%s/%s", project.ID, instance.ID, dbName)

				if reader, found := r.databaseReaders[readerKey]; found {
					activeReaders[readerKey] = reader
					delete(r.databaseReaders, readerKey)
				} else {
					r.logger.Info("Discovered new database, creating reader.", zap.String("reader_key", readerKey))
					dbID := datasource.NewDatabaseID(project.ID, instance.ID, dbName)
					newReader, err := statsreader.NewDatabaseReader(ctx, r.parsedMetadata, dbID, project.ServiceAccountKey, r.readerConfig, r.logger)
					if err != nil {
						allErrors = multierr.Append(allErrors, fmt.Errorf("failed to create reader for %s: %w", readerKey, err))
						continue
					}
					activeReaders[readerKey] = newReader
				}
			}
		}
	}

	for key, staleReader := range r.databaseReaders {
		r.logger.Info("Shutting down reader for removed or un-discoverable database.", zap.String("reader_key", key))
		staleReader.Shutdown()
	}
	r.databaseReaders = activeReaders

	var allMetricsDataPoints []*metadata.MetricsDataPoint
	for key, reader := range r.databaseReaders {
		dataPoints, readErr := reader.Read(ctx)
		if readErr != nil {
			allErrors = multierr.Append(allErrors, fmt.Errorf("error reading from %s: %w", key, readErr))
			continue
		}
		allMetricsDataPoints = append(allMetricsDataPoints, dataPoints...)
	}

	metrics, buildErr := r.metricsBuilder.Build(allMetricsDataPoints)
	if buildErr != nil {
		allErrors = multierr.Append(allErrors, buildErr)
	}

	return metrics, allErrors
}

func (r *googleCloudSpannerReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
	err := r.initialize(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *googleCloudSpannerReceiver) Shutdown(ctx context.Context) error {
	// Shutdown readers created in static mode
	for _, projectReader := range r.projectReaders {
		projectReader.Shutdown()
	}

	// Shutdown readers created in dynamic mode
	r.mu.Lock()
	for _, reader := range r.databaseReaders {
		reader.Shutdown()
	}
	r.mu.Unlock()

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
	var err error
	r.parsedMetadata, err = metadataparser.ParseMetadataConfig(metadataYaml)
	if err != nil {
		return fmt.Errorf("error occurred during parsing of metadata: %w", err)
	}

	// This reader config will be used by both static and dynamic readers
	r.readerConfig = statsreader.ReaderConfig{
		BackfillEnabled:                   r.config.BackfillEnabled,
		TopMetricsQueryMaxRows:            r.config.TopMetricsQueryMaxRows,
		HideTopnLockstatsRowrangestartkey: r.config.HideTopnLockstatsRowrangestartkey,
		TruncateText:                      r.config.TruncateText,
	}

	// The original initialization path is preserved for the static configuration case.
	err = r.initializeProjectReaders(ctx, r.parsedMetadata)
	if err != nil {
		return err
	}

	return r.initializeMetricsBuilder(r.parsedMetadata)
}

// getDatabaseNames returns the list of databases for an instance, either from static config or dynamic discovery.
func (r *googleCloudSpannerReceiver) getDatabaseNames(ctx context.Context, project Project, instance Instance) ([]string, error) {
	if instance.ScrapeAllDatabases {
		r.logger.Debug("ScrapeAllDatabases is true; discovering databases.",
			zap.String("project", project.ID),
			zap.String("instance", instance.ID))
		return r.listDatabasesForInstance(ctx, project.ID, instance.ID, project.ServiceAccountKey)
	}

	r.logger.Debug("Using statically configured database list.",
		zap.String("project", project.ID),
		zap.String("instance", instance.ID))
	return instance.Databases, nil
}

// listDatabasesForInstance uses the Spanner Database Admin Client to fetch all databases for a given instance.
func (r *googleCloudSpannerReceiver) listDatabasesForInstance(ctx context.Context, projectID, instanceID, serviceAccountKey string) ([]string, error) {
	parent := fmt.Sprintf("projects/%s/instances/%s", projectID, instanceID)

	var clientOpts []option.ClientOption
	if serviceAccountKey != "" {
		clientOpts = append(clientOpts, option.WithCredentialsFile(serviceAccountKey))
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create spanner admin client: %w", err)
	}
	defer adminClient.Close()

	re := regexp.MustCompile(fmt.Sprintf(`^%s/databases/(.+)`, parent))
	var databaseNames []string

	it := adminClient.ListDatabases(ctx, &databasepb.ListDatabasesRequest{Parent: parent})
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate through databases: %w", err)
		}

		matches := re.FindStringSubmatch(resp.Name)
		if len(matches) < 2 {
			r.logger.Warn("Could not parse database name from API response", zap.String("name", resp.Name))
			continue
		}
		databaseNames = append(databaseNames, matches[1])
	}

	return databaseNames, nil
}

// ----- Original helper functions preserved for static mode -----

func (r *googleCloudSpannerReceiver) initializeProjectReaders(ctx context.Context,
	parsedMetadata []*metadata.MetricsMetadata,
) error {
	for _, project := range r.config.Projects {
		projectReader, err := newProjectReader(ctx, r.logger, project, parsedMetadata, r.readerConfig)
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
			// In static mode, we count configured databases.
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
	readerConfig statsreader.ReaderConfig,
) (*statsreader.ProjectReader, error) {
	logger.Debug("Constructing project reader for project", zap.String("project id", project.ID))

	var databaseReaders []statsreader.CompositeReader
	for _, instance := range project.Instances {
		// Only create readers for statically configured databases in this path.
		if !instance.ScrapeAllDatabases {
			for _, database := range instance.Databases {
				logger.Debug("Constructing database reader for combination of project, instance, database",
					zap.String("project id", project.ID), zap.String("instance id", instance.ID), zap.String("database", database))

				databaseID := datasource.NewDatabaseID(project.ID, instance.ID, database)

				databaseReader, err := statsreader.NewDatabaseReader(ctx, parsedMetadata, databaseID,
					project.ServiceAccountKey, readerConfig, logger)
				if err != nil {
					return nil, err
				}
				databaseReaders = append(databaseReaders, databaseReader)
			}
		}
	}

	return statsreader.NewProjectReader(databaseReaders, logger), nil
}
