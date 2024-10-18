// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/http"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

type eventSourcePair struct {
	source *componentstatus.InstanceID
	event  *componentstatus.Event
}

type healthCheckExtension struct {
	config        Config
	telemetry     component.TelemetrySettings
	aggregator    *status.Aggregator
	subcomponents []component.Component
	eventCh       chan *eventSourcePair
	readyCh       chan struct{}
	host          component.Host
}

var _ component.Component = (*healthCheckExtension)(nil)
var _ extensioncapabilities.ConfigWatcher = (*healthCheckExtension)(nil)
var _ extensioncapabilities.PipelineWatcher = (*healthCheckExtension)(nil)

func newExtension(
	ctx context.Context,
	config Config,
	set extension.Settings,
) *healthCheckExtension {
	var comps []component.Component

	errPriority := status.PriorityPermanent
	if config.ComponentHealthConfig != nil &&
		config.ComponentHealthConfig.IncludeRecoverable &&
		!config.ComponentHealthConfig.IncludePermanent {
		errPriority = status.PriorityRecoverable
	}

	aggregator := status.NewAggregator(errPriority)

	if config.UseV2 && config.GRPCConfig != nil {
		grpcServer := grpc.NewServer(
			config.GRPCConfig,
			config.ComponentHealthConfig,
			set.TelemetrySettings,
			aggregator,
		)
		comps = append(comps, grpcServer)
	}

	if !config.UseV2 || config.UseV2 && config.HTTPConfig != nil {
		httpServer := http.NewServer(
			config.HTTPConfig,
			config.LegacyConfig,
			config.ComponentHealthConfig,
			set.TelemetrySettings,
			aggregator,
		)
		comps = append(comps, httpServer)
	}

	hc := &healthCheckExtension{
		config:        config,
		subcomponents: comps,
		telemetry:     set.TelemetrySettings,
		aggregator:    aggregator,
		eventCh:       make(chan *eventSourcePair),
		readyCh:       make(chan struct{}),
	}

	// Start processing events in the background so that our status watcher doesn't
	// block others before the extension starts.
	go hc.eventLoop(ctx)

	return hc
}

// Start implements the component.Component interface.
func (hc *healthCheckExtension) Start(ctx context.Context, host component.Host) error {
	hc.telemetry.Logger.Debug("Starting health check extension V2", zap.Any("config", hc.config))

	hc.host = host

	for _, comp := range hc.subcomponents {
		if err := comp.Start(ctx, host); err != nil {
			return err
		}
	}

	return nil
}

// Shutdown implements the component.Component interface.
func (hc *healthCheckExtension) Shutdown(ctx context.Context) error {
	// Preemptively send the stopped event, so it can be exported before shutdown
	componentstatus.ReportStatus(hc.host, componentstatus.NewEvent(componentstatus.StatusStopped))

	close(hc.eventCh)
	hc.aggregator.Close()

	var err error
	for _, comp := range hc.subcomponents {
		err = multierr.Append(err, comp.Shutdown(ctx))
	}

	return err
}

// ComponentStatusChanged implements the extension.StatusWatcher interface.
func (hc *healthCheckExtension) ComponentStatusChanged(
	source *componentstatus.InstanceID,
	event *componentstatus.Event,
) {
	// There can be late arriving events after shutdown. We need to close
	// the event channel so that this function doesn't block and we release all
	// goroutines, but attempting to write to a closed channel will panic; log
	// and recover.
	defer func() {
		if r := recover(); r != nil {
			hc.telemetry.Logger.Info(
				"discarding event received after shutdown",
				zap.Any("source", source),
				zap.Any("event", event),
			)
		}
	}()
	hc.eventCh <- &eventSourcePair{source: source, event: event}
}

// NotifyConfig implements the extensioncapabilities.ConfigWatcher interface.
func (hc *healthCheckExtension) NotifyConfig(ctx context.Context, conf *confmap.Conf) error {
	var err error
	for _, comp := range hc.subcomponents {
		if cw, ok := comp.(extensioncapabilities.ConfigWatcher); ok {
			err = multierr.Append(err, cw.NotifyConfig(ctx, conf))
		}
	}
	return err
}

// Ready implements the extension.PipelineWatcher interface.
func (hc *healthCheckExtension) Ready() error {
	close(hc.readyCh)
	return nil
}

// NotReady implements the extension.PipelineWatcher interface.
func (hc *healthCheckExtension) NotReady() error {
	return nil
}

func (hc *healthCheckExtension) eventLoop(ctx context.Context) {
	// Record events with component.StatusStarting, but queue other events until
	// PipelineWatcher.Ready is called. This prevents aggregate statuses from
	// flapping between StatusStarting and StatusOK as components are started
	// individually by the service.
	var eventQueue []*eventSourcePair

	for loop := true; loop; {
		select {
		case esp, ok := <-hc.eventCh:
			if !ok {
				return
			}
			if esp.event.Status() != componentstatus.StatusStarting {
				eventQueue = append(eventQueue, esp)
				continue
			}
			hc.aggregator.RecordStatus(esp.source, esp.event)
		case <-hc.readyCh:
			for _, esp := range eventQueue {
				hc.aggregator.RecordStatus(esp.source, esp.event)
			}
			eventQueue = nil
			loop = false
		case <-ctx.Done():
			return
		}
	}

	// After PipelineWatcher.Ready, record statuses as they are received.
	for {
		select {
		case esp, ok := <-hc.eventCh:
			if !ok {
				return
			}
			hc.aggregator.RecordStatus(esp.source, esp.event)
		case <-ctx.Done():
			return
		}
	}
}
