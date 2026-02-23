// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

var errNotStarted = errors.New("exporter has not started")

// TODO: Find a place for this to be shared.
type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

// TODO: Find a place for this to be shared.
type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

type signalfMetadataExporter struct {
	exporter.Metrics
	exporter *signalfxExporter
}

func (sme *signalfMetadataExporter) ConsumeMetadata(metadata []*metadata.MetadataUpdate) error {
	return sme.exporter.pushMetadata(metadata)
}

type signalfxExporter struct {
	config                 *Config
	version                string
	logger                 *zap.Logger
	telemetrySettings      component.TelemetrySettings
	pushMetricsData        func(ctx context.Context, md pmetric.Metrics) (droppedTimeSeries int, err error)
	hostMetadataSyncer     *hostmetadata.Syncer
	converter              *translation.MetricsConverter
	dimClient              *dimensions.DimensionClient
	eventClient            *sfxEventClient
	entityEventTransformer *dimensions.EntityEventTransformer
}

// newSignalFxExporter returns a new SignalFx exporter.
func newSignalFxExporter(
	config *Config,
	createSettings exporter.Settings,
) (*signalfxExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	metricTranslator, err := config.getMetricTranslator(make(chan struct{}))
	if err != nil {
		return nil, err
	}

	converter, err := translation.NewMetricsConverter(
		createSettings.Logger,
		metricTranslator,
		config.ExcludeMetrics,
		config.IncludeMetrics,
		config.NonAlphanumericDimensionChars,
		config.DropHistogramBuckets,
		!config.SendOTLPHistograms, // if SendOTLPHistograms is true, do not process histograms when converting to SFx
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric converter: %w", err)
	}

	return &signalfxExporter{
		config:            config,
		version:           createSettings.BuildInfo.Version,
		logger:            createSettings.Logger,
		telemetrySettings: createSettings.TelemetrySettings,
		converter:         converter,
	}, nil
}

func (se *signalfxExporter) start(ctx context.Context, host component.Host) (err error) {
	if se.converter != nil {
		se.converter.Start()
	}
	ingestURL, err := se.config.getIngestURL()
	if err != nil {
		return err
	}

	headers := buildHeaders(se.config, se.version)
	client, err := se.createClient(ctx, host)
	if err != nil {
		return err
	}

	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: ingestURL,
			headers:   headers,
			client:    client,
			zippers:   newGzipPool(),
		},
		logDataPoints:          se.config.LogDataPoints,
		logger:                 se.logger,
		accessTokenPassthrough: se.config.AccessTokenPassthrough,
		converter:              se.converter,
		sendOTLPHistograms:     se.config.SendOTLPHistograms,
	}

	if err := se.startDimensionClient(ctx); err != nil {
		return err
	}

	if se.config.SyncHostMetadata {
		envMap := gopsutilenv.SetGoPsutilEnvVars(se.config.RootPath)
		se.hostMetadataSyncer = hostmetadata.NewSyncer(se.logger, se.dimClient, envMap)
	}
	se.pushMetricsData = dpClient.pushMetricsData

	return nil
}

func (se *signalfxExporter) startDimensionClient(ctx context.Context) error {
	apiTLSCfg, err := se.config.APITLSs.LoadTLSConfig(ctx)
	if err != nil {
		return fmt.Errorf("could not load API TLS config: %w", err)
	}

	apiURL, err := se.config.getAPIURL()
	if err != nil {
		return err
	}

	dimClient := dimensions.NewDimensionClient(
		dimensions.DimensionClientOptions{
			Token:        se.config.AccessToken,
			APIURL:       apiURL,
			APITLSConfig: apiTLSCfg,
			LogUpdates:   se.config.LogDimensionUpdates,
			Logger:       se.logger,
			// Duration to wait between property updates.
			SendDelay:               se.config.DimensionClient.SendDelay,
			MaxBuffered:             se.config.DimensionClient.MaxBuffered,
			NonAlphanumericDimChars: se.config.NonAlphanumericDimensionChars,
			DefaultProperties:       se.config.DefaultProperties,
			ExcludeProperties:       se.config.ExcludeProperties,
			MaxConnsPerHost:         se.config.DimensionClient.MaxConnsPerHost,
			MaxIdleConns:            se.config.DimensionClient.MaxIdleConns,
			MaxIdleConnsPerHost:     se.config.DimensionClient.MaxIdleConnsPerHost,
			IdleConnTimeout:         se.config.DimensionClient.IdleConnTimeout,
			Timeout:                 se.config.DimensionClient.Timeout,
			DropTags:                se.config.DimensionClient.DropTags,
		})
	dimClient.Start()
	se.dimClient = dimClient
	return nil
}

func newGzipPool() sync.Pool {
	return sync.Pool{New: func() any {
		return gzip.NewWriter(nil)
	}}
}

func newEventExporter(config *Config, createSettings exporter.Settings) (*signalfxExporter, error) {
	if config == nil {
		return nil, errors.New("nil config")
	}

	return &signalfxExporter{
		config:                 config,
		version:                createSettings.BuildInfo.Version,
		logger:                 createSettings.Logger,
		telemetrySettings:      createSettings.TelemetrySettings,
		entityEventTransformer: dimensions.NewEntityEventTransformer(config.DefaultProperties),
	}, nil
}

func (se *signalfxExporter) startLogs(ctx context.Context, host component.Host) error {
	ingestURL, err := se.config.getIngestURL()
	if err != nil {
		return err
	}

	headers := buildHeaders(se.config, se.version)
	client, err := se.createClient(ctx, host)
	if err != nil {
		return err
	}

	eventClient := &sfxEventClient{
		sfxClientBase: sfxClientBase{
			ingestURL: ingestURL,
			headers:   headers,
			client:    client,
			zippers:   newGzipPool(),
		},
		logger:                 se.logger,
		accessTokenPassthrough: se.config.AccessTokenPassthrough,
	}

	// Initialize dimension client for entity event processing if entity events processing is enabled.
	if entityEventsFeatureGate.IsEnabled() {
		if err := se.startDimensionClient(ctx); err != nil {
			return err
		}
	}
	se.eventClient = eventClient
	return nil
}

func (se *signalfxExporter) createClient(ctx context.Context, host component.Host) (*http.Client, error) {
	se.config.TLS = se.config.IngestTLSs

	return se.config.ToClient(ctx, host.GetExtensions(), se.telemetrySettings)
}

func (se *signalfxExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	_, err := se.pushMetricsData(ctx, md)
	if err == nil && se.hostMetadataSyncer != nil {
		se.hostMetadataSyncer.Sync(md)
	}
	return err
}

func (se *signalfxExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	rls := ld.ResourceLogs()
	if rls.Len() == 0 {
		return nil
	}

	var sfxEvents []*sfxpb.Event

	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			sl := ills.At(j)

			// Process logs that represent entity events and skip regular event conversion
			if entityEventsFeatureGate.IsEnabled() && isEntityEventScope(sl) {
				err := se.processEntityEvents(sl.LogRecords())
				if err != nil {
					return fmt.Errorf("failed to process entity events: %w", err)
				}
				continue
			}

			events, _ := translation.LogRecordSliceToSignalFxV2(se.logger, sl.LogRecords(), rl.Resource().Attributes())
			sfxEvents = append(sfxEvents, events...)
		}
	}

	if len(sfxEvents) > 0 {
		err := se.eventClient.pushEvents(ctx, rls.At(0), sfxEvents)
		if err != nil {
			return fmt.Errorf("failed to push events: %w", err)
		}
	}

	return nil
}

func (se *signalfxExporter) processEntityEvents(logs plog.LogRecordSlice) error {
	entityEventsSlice := metadata.NewEntityEventsSliceFromLogs(logs)
	for i := 0; i < entityEventsSlice.Len(); i++ {
		entityEvent := entityEventsSlice.At(i)

		dimUpdate, err := se.entityEventTransformer.TransformEntityEvent(entityEvent)
		if dimUpdate == nil {
			if err != nil {
				se.logger.Warn("Failed to transform entity event to dimension update", zap.Error(err))
			}
			continue
		}

		// Failure to accept dimension likely means exceeded buffer. Reject all further
		// dimension updates for this export cycle.
		if err := se.dimClient.AcceptDimension(dimUpdate); err != nil {
			return fmt.Errorf("failed to accept dimension update: %w", err)
		}
	}
	return nil
}

func (se *signalfxExporter) shutdown(_ context.Context) error {
	if se.dimClient != nil {
		se.dimClient.Shutdown()
	}

	if se.converter != nil {
		se.converter.Shutdown()
	}
	return nil
}

func (se *signalfxExporter) pushMetadata(metadata []*metadata.MetadataUpdate) error {
	if se.dimClient == nil {
		return errNotStarted
	}
	return se.dimClient.PushMetadata(metadata)
}

func isEntityEventScope(sl plog.ScopeLogs) bool {
	marker, ok := sl.Scope().Attributes().Get(metadata.SemconvOtelEntityEventAsScope)
	return ok && marker.Type() == pcommon.ValueTypeBool && marker.Bool()
}

func buildHeaders(config *Config, version string) map[string]string {
	headers := map[string]string{
		"Connection":   "keep-alive",
		"Content-Type": "application/x-protobuf",
		"User-Agent":   fmt.Sprintf("OpenTelemetry-Collector SignalFx Exporter/%s", version),
	}

	if config.AccessToken != "" {
		headers[splunk.SFxAccessTokenHeader] = string(config.AccessToken)
	}

	// Add any custom headers from the config. They will override the pre-defined
	// ones above in case of conflict, but, not the content encoding one since
	// the latter one is defined according to the payload.
	for k, v := range config.Headers.Iter {
		headers[k] = string(v)
	}
	// we want to control how headers are set, overriding user headers with our passthrough.
	config.Headers = nil

	return headers
}
