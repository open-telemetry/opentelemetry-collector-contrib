// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"
	"sync"
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

const payloadSendingInterval = 30 * time.Minute

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

type configs struct {
	collector *confmap.Conf
	extension *Config
	mutex     sync.RWMutex
}

type info struct {
	host           source.Source
	hostnameSource string
	uuid           string
	build          component.BuildInfo
	modules        service.ModuleInfos
}

type payloadSender struct {
	ticker  *time.Ticker
	ctx     context.Context
	cancel  context.CancelFunc
	channel chan struct{}
	once    sync.Once
}

type datadogExtension struct {
	extension.Extension // Embed base Extension for common functionality.

	// struct to store extension and collector configs
	configs *configs

	// components to assist in payload sending/logging
	logger     *zap.Logger
	serializer agentcomponents.SerializerWithForwarder

	// struct to store extension info
	info *info

	otelCollectorMetadata *payload.OtelCollector
	httpServer            *httpserver.Server

	// Fields for periodic payload sending
	payloadSender *payloadSender
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
	e.configs.mutex.Lock()
	defer e.configs.mutex.Unlock()

	e.configs.collector = conf

	// Create the build info struct for the payload
	buildInfo := payload.CustomBuildInfo{
		Command:     e.info.build.Command,
		Description: e.info.build.Description,
		Version:     e.info.build.Version,
	}

	// Get the full collector configuration as a flattened JSON string
	fullConfig := componentchecker.DataToFlattenedJSONString(e.configs.collector.ToStringMap())

	// Prepare the base payload
	otelCollectorPayload := payload.PrepareOtelCollectorMetadata(
		e.info.host.Identifier,
		e.info.hostnameSource,
		e.info.uuid,
		e.info.build.Version, // This is the same version from buildInfo; it is possible we could want to set a different version here in the future
		e.configs.extension.API.Site,
		fullConfig,
		buildInfo,
	)

	// Populate the full list of components available in the collector build
	moduleInfoJSON, err := componentchecker.PopulateFullComponentsJSON(e.info.modules, e.configs.collector)
	if err != nil {
		e.logger.Warn("Failed to populate full components list", zap.Error(err))
	} else {
		otelCollectorPayload.FullComponents = moduleInfoJSON.GetFullComponentsList()
	}

	// Populate the list of components that are active in a pipeline
	activeComponents, err := componentchecker.PopulateActiveComponents(e.configs.collector, moduleInfoJSON)
	if err != nil {
		e.logger.Warn("Failed to populate active components list", zap.Error(err))
	} else if activeComponents != nil {
		otelCollectorPayload.ActiveComponents = *activeComponents
	}

	// TODO: Populate HealthStatus from the pkg/status.
	// https://datadoghq.atlassian.net/browse/OTEL-2663
	// For now, we leave it as an empty string.
	otelCollectorPayload.HealthStatus = "{}"

	// Store the created payload in the extension struct
	e.otelCollectorMetadata = &otelCollectorPayload
	e.logger.Debug("Datadog extension payload created", zap.Any("payload", e.otelCollectorMetadata))

	// Create and start the HTTP server
	e.httpServer = httpserver.NewServer(
		e.logger,
		e.serializer,
		e.configs.extension.HTTPConfig,
		e.info.host.Identifier,
		e.info.uuid,
		otelCollectorPayload,
	)
	e.httpServer.Start()
	_, err = e.httpServer.SendPayload()
	if err != nil {
		return err
	}

	// Start periodic payload sending (every 30 minutes)
	e.startPeriodicPayloadSending()

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
		e.info.modules = mi.GetModuleInfos()
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

	// Stop periodic payload sending
	e.stopPeriodicPayloadSending()

	if e.httpServer != nil {
		e.httpServer.Stop(ctxWithTimeout)
	}
	// Stop the serializer if it's available
	if e.serializer != nil {
		e.serializer.Stop()
	}
	return nil
}

// startPeriodicPayloadSending starts a goroutine that sends payloads on payloadSendingInterval
func (e *datadogExtension) startPeriodicPayloadSending() {
	e.payloadSender.once.Do(func() {
		go func() {
			defer e.payloadSender.ticker.Stop()
			for {
				select {
				case <-e.payloadSender.ticker.C:
					e.logger.Debug("Sending periodic payload to Datadog")
					if _, err := e.httpServer.SendPayload(); err != nil {
						e.logger.Error("Failed to send periodic payload", zap.Error(err))
					} else {
						e.logger.Debug("Successfully sent periodic payload to Datadog")
					}
				case <-e.payloadSender.ctx.Done():
					e.logger.Debug("Stopping periodic payload sending")
					return
				case <-e.payloadSender.channel:
					// Allow manual triggering of payload sending if needed
					e.logger.Debug("Manually triggered payload send")
					if _, err := e.httpServer.SendPayload(); err != nil {
						e.logger.Error("Failed to send manually triggered payload", zap.Error(err))
					}
				}
			}
		}()

		e.logger.Info("Started periodic payload sending", zap.Duration("interval", payloadSendingInterval))
	})
}

// stopPeriodicPayloadSending stops the periodic payload sending goroutine
func (e *datadogExtension) stopPeriodicPayloadSending() {
	if e.payloadSender == nil {
		return
	}
	if e.payloadSender.cancel != nil {
		e.payloadSender.cancel()
	}
	if e.payloadSender.ticker != nil {
		e.payloadSender.ticker.Stop()
	}
	if e.payloadSender.channel != nil {
		close(e.payloadSender.channel)
	}
}

// GetSerializer returns the configured serializer with proxy settings from ClientConfig.
// This allows other components (like exporters) to use the same serializer instance
// with the same proxy configuration.
func (e *datadogExtension) GetSerializer() agentcomponents.SerializerWithForwarder {
	return e.serializer
}

func newExtension(
	ctx context.Context,
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
	var hostnameSource string
	if cfg.Hostname != "" {
		hostnameSource = "config"
	} else {
		hostnameSource = "inferred"
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

	// configure payloadSender struct
	ctxWithCancel, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(payloadSendingInterval)
	channel := make(chan struct{}, 1)

	e := &datadogExtension{
		configs:    &configs{extension: cfg},
		logger:     set.Logger,
		serializer: serializer,
		info: &info{
			host:           host,
			hostnameSource: hostnameSource,
			uuid:           uuidProvider.NewString(),
			build:          set.BuildInfo,
			modules:        service.ModuleInfos{}, // moduleInfos will be populated in Start()
		},
		payloadSender: &payloadSender{
			ctx:     ctxWithCancel,
			cancel:  cancel,
			ticker:  ticker,
			channel: channel,
		},
	}
	return e, nil
}
