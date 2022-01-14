package mongodbreceiver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type mongodbScraper struct {
	logger    *zap.Logger
	config    *Config
	client    client
	extractor *extractor
}

func newMongodbScraper(logger *zap.Logger, config *Config) (*mongodbScraper, error) {
	clientLogger := logger.Named("mongo-scraper")
	c, err := NewClient(config, clientLogger)
	if err != nil {
		return nil, fmt.Errorf("unable to start mongodb receiver: %w", err)
	}

	return &mongodbScraper{
		logger: logger,
		config: config,
		client: c,
	}, nil
}

func (s *mongodbScraper) start(ctx context.Context, host component.Host) error {
	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("unable to connect to mongo instance: %w", err)
	}

	vr, err := s.client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("unable to get a version from the mongo instance: %w", err)
	}

	s.extractor, err = newExtractor(vr.Version, s.logger)
	if err != nil {
		return err
	}

	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

func (s *mongodbScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	s.logger.Debug("starting mongoDB scrape")
	if s.client == nil {
		return pdata.NewMetrics(), errors.New("no client was initialized before calling scrape")
	}
	return s.collectMetrics(ctx, s.client)
}

func (s *mongodbScraper) collectMetrics(ctx context.Context, client client) (pdata.Metrics, error) {
	rms := pdata.NewMetrics()
	ilm := rms.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otelcol/mongodb")
	mm := newMetricManager(s.logger, ilm)
	dbNames, err := client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		s.logger.Error("Failed to fetch database names", zap.Error(err))
		return pdata.NewMetrics(), err
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go s.collectAdminDatabase(ctx, wg, mm, client)

	for _, dbName := range dbNames {
		wg.Add(1)
		go s.collectDatabase(ctx, wg, mm, client, dbName)
	}

	wg.Wait()

	return rms, nil
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, wg *sync.WaitGroup, mm *metricManager, client client, databaseName string) {
	defer wg.Done()
	dbStats, err := client.DBStats(ctx, databaseName)
	if err != nil {
		s.logger.Error("Failed to collect dbStats metric", zap.Error(err), zap.String("database", databaseName))
	} else {
		s.extractor.Extract(dbStats, mm, databaseName, normalDBStats)
	}

	serverStatus, err := client.ServerStatus(ctx, databaseName)
	if err != nil {
		s.logger.Error("Failed to collect serverStatus metric", zap.Error(err), zap.String("database", databaseName))
	} else {
		s.extractor.Extract(serverStatus, mm, databaseName, normalServerStats)
	}
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, wg *sync.WaitGroup, mm *metricManager, client client) {
	defer wg.Done()
	serverStatus, err := client.ServerStatus(ctx, "admin")
	if err != nil {
		s.logger.Error("Failed to query serverStatus on admin database", zap.Error(err))
	} else {
		s.extractor.Extract(serverStatus, mm, "admin", adminServerStats)
	}
}

type metricManager struct {
	logger             *zap.Logger
	ilm                pdata.InstrumentationLibraryMetrics
	initializedMetrics map[string]pdata.Metric
	mutex              *sync.RWMutex
	now                pdata.Timestamp
}

func newMetricManager(logger *zap.Logger, ilm pdata.InstrumentationLibraryMetrics) *metricManager {
	mutex := &sync.RWMutex{}
	return &metricManager{
		logger:             logger,
		ilm:                ilm,
		initializedMetrics: map[string]pdata.Metric{},
		mutex:              mutex,
		now:                pdata.NewTimestampFromTime(time.Now()),
	}
}

func (m *metricManager) addDataPoint(metricDef metadata.MetricIntf, value interface{}, attributes pdata.AttributeMap) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var dataPoint pdata.NumberDataPoint
	switch v := value.(type) {
	case int64:
		currDatapoints := m.getOrInit(metricDef)
		dataPoint = currDatapoints.AppendEmpty()
		dataPoint.SetTimestamp(m.now)
		dataPoint.SetIntVal(v)
	case float64:
		currDatapoints := m.getOrInit(metricDef)
		dataPoint = currDatapoints.AppendEmpty()
		dataPoint.SetTimestamp(m.now)
		dataPoint.SetDoubleVal(v)
	default:
		m.logger.Warn(fmt.Sprintf("unknown metric data type for metric: %s", metricDef.Name()))
		return
	}
	attributes.CopyTo(dataPoint.Attributes())
}

func (m *metricManager) getOrInit(metricDef metadata.MetricIntf) pdata.NumberDataPointSlice {
	metric, ok := m.initializedMetrics[metricDef.Name()]
	if !ok {
		metric = m.ilm.Metrics().AppendEmpty()
		metricDef.Init(metric)
		m.initializedMetrics[metricDef.Name()] = metric
	}

	if metric.DataType() == pdata.MetricDataTypeSum {
		return metric.Sum().DataPoints()
	}

	if metric.DataType() == pdata.MetricDataTypeGauge {
		return metric.Gauge().DataPoints()
	}

	m.logger.Error("Failed to get or init metric of unknown type", zap.String("metric", metricDef.Name()))
	return pdata.NewNumberDataPointSlice()
}
