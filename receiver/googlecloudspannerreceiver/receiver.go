// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver"

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sync"
	"time"

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

// newReaderFunc is a function type that creates a new database reader.
// It's defined as a type to make it mockable in tests.
type newReaderFunc func(ctx context.Context, parsedMetadata []*metadata.MetricsMetadata, databaseID *datasource.DatabaseID, serviceAccountPath string, readerConfig statsreader.ReaderConfig, logger *zap.Logger) (statsreader.CompositeReader, error)

// listDatabasesFunc is a function type for listing databases.
// It's defined as a type to make it mockable in tests.
type listDatabasesFunc func(ctx context.Context, projectID, instanceID, serviceAccountKey string) ([]string, error)

type googleCloudSpannerReceiver struct {
	logger            *zap.Logger
	config            *Config
	cancel            context.CancelFunc
	projectReaders    []*statsreader.ProjectReader // Readers for statically configured instances
	metricsBuilder    metadata.MetricsBuilder
	readerConfig      statsreader.ReaderConfig
	parsedMetadata    []*metadata.MetricsMetadata
	lastDiscoveryTime time.Time // Tracks the last time a dynamic discovery was performed.

	// mu protects access to the databaseReaders map in dynamic mode.
	mu sync.Mutex
	// databaseReaders holds the readers for dynamically discovered databases.
	databaseReaders map[string]statsreader.CompositeReader

	// listDatabasesFunc is a function field that can be replaced in tests for mocking.
	listDatabasesFunc listDatabasesFunc
	// newDbReaderFunc is a function field that can be replaced in tests for mocking.
	newDbReaderFunc newReaderFunc
}

func newGoogleCloudSpannerReceiver(logger *zap.Logger, config *Config) *googleCloudSpannerReceiver {
	r := &googleCloudSpannerReceiver{
		logger:          logger,
		config:          config,
		databaseReaders: make(map[string]statsreader.CompositeReader),
	}

	// In production, these fields will point to the real implementations.
	r.listDatabasesFunc = r.listDatabasesForInstance
	r.newDbReaderFunc = func(ctx context.Context, parsedMetadata []*metadata.MetricsMetadata, databaseID *datasource.DatabaseID, serviceAccountPath string, readerConfig statsreader.ReaderConfig, logger *zap.Logger) (statsreader.CompositeReader, error) {
		return statsreader.NewDatabaseReader(ctx, parsedMetadata, databaseID, serviceAccountPath, readerConfig, logger)
	}

	return r
}

func (r *googleCloudSpannerReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	var allMetricsDataPoints []*metadata.MetricsDataPoint
	var allErrors error

	// 1. Scrape all statically configured instances
	for _, projectReader := range r.projectReaders {
		dataPoints, readErr := projectReader.Read(ctx)
		if readErr != nil {
			allErrors = multierr.Append(allErrors, readErr)
		}
		allMetricsDataPoints = append(allMetricsDataPoints, dataPoints...)
	}

	// 2. Scrape all dynamically configured instances
	dynamicDataPoints, dynamicErr := r.scrapeDynamicInstances(ctx)
	if dynamicErr != nil {
		allErrors = multierr.Append(allErrors, dynamicErr)
	}
	allMetricsDataPoints = append(allMetricsDataPoints, dynamicDataPoints...)

	// 3. Build metrics from all collected data points
	metrics, buildErr := r.metricsBuilder.Build(allMetricsDataPoints)
	if buildErr != nil {
		allErrors = multierr.Append(allErrors, buildErr)
	}

	if allErrors != nil && metrics.DataPointCount() > 0 {
		allErrors = scrapererror.NewPartialScrapeError(allErrors, len(multierr.Errors(allErrors)))
	}

	return metrics, allErrors
}

// scrapeDynamicInstances handles the discovery and scraping for all instances marked for dynamic scraping.
func (r *googleCloudSpannerReceiver) scrapeDynamicInstances(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Determine if it's time to run a periodic discovery.
	shouldDiscover := false
	if r.config.DiscoveryInterval > 0 {
		if time.Since(r.lastDiscoveryTime) > r.config.DiscoveryInterval {
			shouldDiscover = true
		}
	}

	if shouldDiscover {
		r.logger.Info("Running periodic database discovery.", zap.Duration("interval", r.config.DiscoveryInterval))
		r.lastDiscoveryTime = time.Now()
		var discoveryErrors error

		// Run a full discovery and reconciliation of readers
		discoveredReaders := make(map[string]bool)
		for _, project := range r.config.Projects {
			for _, instance := range project.Instances {
				if !instance.ScrapeAllDatabases {
					continue
				}

				databaseNames, err := r.listDatabasesFunc(ctx, project.ID, instance.ID, project.ServiceAccountKey)
				if err != nil {
					discoveryErrors = multierr.Append(discoveryErrors, fmt.Errorf("failed to list databases for instance %s/%s: %w", project.ID, instance.ID, err))
					continue
				}

				for _, dbName := range databaseNames {
					readerKey := fmt.Sprintf("%s/%s/%s", project.ID, instance.ID, dbName)
					discoveredReaders[readerKey] = true
					if _, found := r.databaseReaders[readerKey]; !found {
						r.logger.Info("Discovered new database, creating reader.", zap.String("reader_key", readerKey))
						dbID := datasource.NewDatabaseID(project.ID, instance.ID, dbName)
						newReader, err := r.newDbReaderFunc(ctx, r.parsedMetadata, dbID, project.ServiceAccountKey, r.readerConfig, r.logger)
						if err != nil {
							discoveryErrors = multierr.Append(discoveryErrors, fmt.Errorf("failed to create reader for %s: %w", readerKey, err))
							continue
						}
						r.databaseReaders[readerKey] = newReader
					}
				}
			}
		}

		// Shutdown any stale readers that were not found in this discovery cycle.
		for key, reader := range r.databaseReaders {
			if !discoveredReaders[key] {
				r.logger.Info("Shutting down reader for removed database.", zap.String("reader_key", key))
				reader.Shutdown()
				delete(r.databaseReaders, key)
			}
		}

		if discoveryErrors != nil {
			r.logger.Error("Errors encountered during database discovery", zap.Error(discoveryErrors))
		}
	} else {
		r.logger.Debug("Skipping periodic database discovery based on interval.")
	}

	// Read from all currently active dynamic readers
	var allMetricsDataPoints []*metadata.MetricsDataPoint
	var readErrors error
	for key, reader := range r.databaseReaders {
		dataPoints, err := reader.Read(ctx)
		if err != nil {
			readErrors = multierr.Append(readErrors, fmt.Errorf("error reading from %s: %w", key, err))
			continue
		}
		allMetricsDataPoints = append(allMetricsDataPoints, dataPoints...)
	}

	return allMetricsDataPoints, readErrors
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
	// Shutdown readers created for static mode
	for _, projectReader := range r.projectReaders {
		projectReader.Shutdown()
	}

	// Shutdown readers created for dynamic mode
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

// initialize now performs the initial discovery for dynamic instances.
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

	// Initialize readers for statically configured instances ONLY.
	err = r.initializeStaticProjectReaders(ctx, r.parsedMetadata)
	if err != nil {
		return err
	}

	// Perform initial discovery for dynamically configured instances.
	err = r.initializeDynamicDatabaseReaders(ctx)
	if err != nil {
		return err
	}

	return r.initializeMetricsBuilder(r.parsedMetadata)
}

// initializeDynamicDatabaseReaders performs the first discovery at startup.
func (r *googleCloudSpannerReceiver) initializeDynamicDatabaseReaders(ctx context.Context) error {
	r.logger.Info("Performing initial database discovery for dynamic instances.")
	var allErrors error

	for _, project := range r.config.Projects {
		for _, instance := range project.Instances {
			if instance.ScrapeAllDatabases {
				databaseNames, err := r.listDatabasesFunc(ctx, project.ID, instance.ID, project.ServiceAccountKey)
				if err != nil {
					allErrors = multierr.Append(allErrors, fmt.Errorf("failed initial discovery for instance %s/%s: %w", project.ID, instance.ID, err))
					continue
				}

				for _, dbName := range databaseNames {
					readerKey := fmt.Sprintf("%s/%s/%s", project.ID, instance.ID, dbName)
					r.logger.Info("Initial discovery: creating reader.", zap.String("reader_key", readerKey))
					dbID := datasource.NewDatabaseID(project.ID, instance.ID, dbName)
					newReader, err := r.newDbReaderFunc(ctx, r.parsedMetadata, dbID, project.ServiceAccountKey, r.readerConfig, r.logger)
					if err != nil {
						allErrors = multierr.Append(allErrors, fmt.Errorf("failed to create initial reader for %s: %w", readerKey, err))
						continue
					}
					r.databaseReaders[readerKey] = newReader
				}
			}
		}
	}

	// Set the discovery time so the next discovery respects the interval.
	r.lastDiscoveryTime = time.Now()

	return allErrors
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

// initializeStaticProjectReaders creates ProjectReaders ONLY for statically configured instances.
func (r *googleCloudSpannerReceiver) initializeStaticProjectReaders(ctx context.Context,
	parsedMetadata []*metadata.MetricsMetadata,
) error {
	for _, project := range r.config.Projects {
		projectReader, err := newStaticProjectReader(ctx, r.logger, project, parsedMetadata, r.readerConfig)
		if err != nil {
			return err
		}
		// newStaticProjectReader will return nil if there are no static instances, so we check.
		if projectReader != nil {
			r.projectReaders = append(r.projectReaders, projectReader)
		}
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
			// In static mode, we count configured databases for a baseline cardinality limit.
			// Dynamic instances are not counted here as their number can change.
			if !instance.ScrapeAllDatabases {
				databaseAmount += len(instance.Databases)
			}
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

// newStaticProjectReader creates a ProjectReader containing readers ONLY for statically configured instances within a project.
// If a project has no statically configured instances, it returns nil.
func newStaticProjectReader(ctx context.Context, logger *zap.Logger, project Project, parsedMetadata []*metadata.MetricsMetadata,
	readerConfig statsreader.ReaderConfig,
) (*statsreader.ProjectReader, error) {
	logger.Debug("Constructing project reader for static instances in project", zap.String("project id", project.ID))

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

	// If no static readers were created for this project, there's nothing to do.
	if len(databaseReaders) == 0 {
		return nil, nil
	}

	return statsreader.NewProjectReader(databaseReaders, logger), nil
}
