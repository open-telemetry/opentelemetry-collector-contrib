// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"errors"
	"time"

	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// uuidProvider defines an interface for generating UUIDs, allowing for mocking in tests.
type uuidProvider interface {
	NewString() string
}

// realUUIDProvider is the concrete implementation of uuidProvider that uses the google/uuid package.
type realUUIDProvider struct{}

// NewString returns a new UUID string using the real uuid package.
func (p *realUUIDProvider) NewString() string {
	return uuid.New().String()
}

type datadogExtension struct {
	extension.Extension   // Embed base Extension for common functionality.
	collectorConfig       *confmap.Conf
	extensionConfig       *Config
	logger                *zap.Logger
	serializer            agentcomponents.SerializerWithForwarder
	host                  source.Source
	uuid                  string
	buildInfo             component.BuildInfo
	moduleInfos           service.ModuleInfos // NOTE: This needs to be populated from the service if available
	otelCollectorMetadata *payload.OtelCollector
	httpServer            *httpserver.Server
}

var (
	_ extensioncapabilities.ConfigWatcher   = (*datadogExtension)(nil)
	_ extensioncapabilities.PipelineWatcher = (*datadogExtension)(nil)
	_ componentstatus.Watcher               = (*datadogExtension)(nil)
)

// NotifyConfig implements the extensioncapabilities.ConfigWatcher interface, which allows
// this extension to be notified of the Collector's effective configuration.
// This method is called during startup by the Collector's service after calling Start.
func (e *datadogExtension) NotifyConfig(_ context.Context, conf *confmap.Conf) error {
	e.collectorConfig = conf

	// Create the build info struct for the payload
	buildInfo := payload.CustomBuildInfo{
		Command:     e.buildInfo.Command,
		Description: e.buildInfo.Description,
		Version:     e.buildInfo.Version,
	}

	// Get the full collector configuration as a flattened JSON string
	fullConfig := componentchecker.DataToFlattenedJSONString(conf.ToStringMap())
	var hostnameSource string
	if e.extensionConfig.Hostname != "" {
		hostnameSource = "config"
	} else {
		hostnameSource = "inferred"
	}
	// Prepare the base payload
	otelCollectorPayload := payload.PrepareOtelCollectorMetadata(
		e.host.Identifier,
		hostnameSource,
		e.uuid,
		e.buildInfo.Version,
		e.extensionConfig.API.Site,
		fullConfig,
		buildInfo,
	)

	// Populate the full list of components available in the collector build
	moduleInfoJSON, err := componentchecker.PopulateFullComponentsJSON(e.moduleInfos, conf)
	if err != nil {
		e.logger.Warn("Failed to populate full components list", zap.Error(err))
	} else {
		otelCollectorPayload.FullComponents = moduleInfoJSON.GetFullComponentsList()
	}

	// Populate the list of components that are active in a pipeline
	activeComponents, err := componentchecker.PopulateActiveComponents(conf, moduleInfoJSON)
	if err != nil {
		e.logger.Warn("Failed to populate active components list", zap.Error(err))
	} else if activeComponents != nil {
		otelCollectorPayload.ActiveComponents = *activeComponents
	}

	// TODO: Populate HealthStatus from the pkg/status.
	// For now, we leave it as an empty string.
	otelCollectorPayload.HealthStatus = "{}"

	// Store the created payload in the extension struct
	e.otelCollectorMetadata = &otelCollectorPayload
	e.logger.Info("Datadog extension payload created", zap.Any("payload", e.otelCollectorMetadata))
	if e.extensionConfig.HTTPConfig == nil {
		return errors.New("local HTTP server config is required to send payloads to Datadog")
	}
	// Create and start the HTTP server
	e.httpServer = httpserver.NewServer(
		e.logger,
		e.serializer,
		e.extensionConfig.HTTPConfig,
		e.host.Identifier,
		e.uuid,
		otelCollectorPayload,
	)
	e.httpServer.Start()
	_, err = e.httpServer.SendPayload()
	if err != nil {
		return err
	}
	return nil
}

// Ready implements the extensioncapabilities.PipelineWatcher interface.
func (e *datadogExtension) Ready() error {
	return nil
}

// NotReady implements the extensioncapabilities.PipelineWatcher interface.
func (e *datadogExtension) NotReady() error {
	return nil
}

// ComponentStatusChanged implements the componentstatus.Watcher interface.
func (e *datadogExtension) ComponentStatusChanged(
	*componentstatus.InstanceID,
	*componentstatus.Event,
) {
}

// Start starts the extension via the component interface.
func (e *datadogExtension) Start(_ context.Context, host component.Host) error {
	// Retrieve module information from the host
	if mi, ok := host.(hostcapabilities.ModuleInfo); ok {
		e.moduleInfos = mi.GetModuleInfos()
	} else {
		e.logger.Warn("Host does not implement hostcapabilities.ModuleInfo, component list in payload will be empty.")
	}

	// Start the serializer if it's available
	if e.serializer != nil {
		return e.serializer.Start()
	}
	return nil
}

// Shutdown stops the extension via the component interface.
// It shuts down the HTTP server, stops forwarder, and passes signal on
// channel to end goroutine that sends the Datadog otel_collector payloads.
func (e *datadogExtension) Shutdown(ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if e.httpServer != nil {
		e.httpServer.Stop(ctxWithTimeout)
	}
	// Stop the serializer if it's available
	if e.serializer != nil {
		e.serializer.Stop()
	}
	return nil
}

// GetSerializer returns the configured serializer with proxy settings from ClientConfig.
// This allows other components (like exporters) to use the same serializer instance
// with the same proxy configuration.
func (e *datadogExtension) GetSerializer() agentcomponents.SerializerWithForwarder {
	return e.serializer
}

func newExtension(
	_ context.Context,
	cfg *Config,
	set extension.Settings,
	hostProvider source.Provider,
	uuidProvider uuidProvider,
) (*datadogExtension, error) {
	// Create configuration for agent components
	// Convert datadogextension.Config to datadogconfig.Config
	ddConfig := &datadogconfig.Config{
		API:          cfg.API,
		ClientConfig: cfg.ClientConfig,
	}
	host, err := hostProvider.Source(context.Background())
	if err != nil {
		return nil, err
	}

	// Create agent components with proxy configuration from ClientConfig
	configOptions := []agentcomponents.ConfigOption{
		agentcomponents.WithAPIConfig(ddConfig),
		agentcomponents.WithForwarderConfig(),
		agentcomponents.WithPayloadsConfig(),
		// Use ClientConfig proxy settings instead of environment variables
		agentcomponents.WithProxy(ddConfig),
		// logging_frequency required to be set to avoid "divide by zero" error
		agentcomponents.WithCustomConfig("logging_frequency", 1, pkgconfigmodel.SourceDefault),
	}

	// Create agent components
	configComponent := agentcomponents.NewConfigComponent(configOptions...)
	logComponent := agentcomponents.NewLogComponent(set.TelemetrySettings)
	serializer := agentcomponents.NewSerializerComponent(configComponent, logComponent, host.Identifier)

	e := &datadogExtension{
		extensionConfig: cfg,
		logger:          set.Logger,
		serializer:      serializer,
		host:            host,
		moduleInfos:     service.ModuleInfos{}, // will be populated in Start
		uuid:            uuidProvider.NewString(),
		buildInfo:       set.BuildInfo,
	}
	return e, nil
}
