// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver

import (
	"context"
	"sync"
	"time"

	gnmi "github.com/openconfig/gnmi/proto/gnmi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnmireceiver/internal"
)

// gnmiReceiver manages gNMI dial-in sessions for all configured targets.
type gnmiReceiver struct {
	cfg          *Config
	settings     receiver.Settings
	nextConsumer consumer.Metrics
	logger       *zap.Logger
	cancel       context.CancelFunc
	wg           *sync.WaitGroup
	yangParser   *internal.YANGParser
}

// newGNMIReceiver creates a new instance of the gNMI receiver.
func newGNMIReceiver(
	cfg *Config,
	set receiver.Settings,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	// Initialise the YANG parser and load built-in modules.
	// External .yang files are loaded in Start() once module_paths are known.
	yangParser := internal.NewYANGParser()
	yangParser.LoadBuiltinModules()

	return &gnmiReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		logger:       set.Logger,
		wg:           &sync.WaitGroup{},
		yangParser:   yangParser,
	}, nil
}

// Start begins the gNMI subscriptions for all configured targets.
// It creates a dedicated goroutine for each network device to ensure parallel processing.
func (r *gnmiReceiver) Start(_ context.Context, _ component.Host) error {
	r.logger.Info("Starting gNMI receiver")

	// Load optional external YANG files for improved Counter/Gauge resolution (level 1).
	// Files are loaded once here so all target goroutines share the same parser instance.
	for _, path := range r.cfg.ModulePaths {
		if err := r.yangParser.ExtractYANGFromFiles(path); err != nil {
			r.logger.Warn("Failed to load YANG modules from path",
				zap.String("path", path),
				zap.Error(err))
		}
	}

	// Create a dedicated background context for subscriptions.
	// This context persists as long as the receiver is running.
	var subCtx context.Context
	subCtx, r.cancel = context.WithCancel(context.Background())

	// Launch a worker goroutine per target device (Cisco IOS-XE, NX-OS, Arista EOS).
	for i := range r.cfg.Targets {
		target := r.cfg.Targets[i]
		r.wg.Add(1)
		go func(t TargetConfig) {
			defer r.wg.Done()
			r.startSubscription(subCtx, t)
		}(target)
	}

	return nil
}

// Shutdown gracefully stops all active gNMI sessions.
func (r *gnmiReceiver) Shutdown(_ context.Context) error {
	r.logger.Info("Shutting down gNMI receiver")

	if r.cancel != nil {
		r.cancel()
	}

	r.wg.Wait()
	return nil
}

// startSubscription handles the lifecycle of a single gNMI target connection.
func (r *gnmiReceiver) startSubscription(ctx context.Context, target TargetConfig) {
	r.logger.Debug("Initiating gNMI session", zap.String("target", target.Address))

	encoding := gnmi.Encoding_PROTO
	switch target.Encoding {
	case "json":
		encoding = gnmi.Encoding_JSON
	case "json_ietf":
		encoding = gnmi.Encoding_JSON_IETF
	}

	client := &gnmiClient{
		target:   target,
		encoding: encoding,
		consumer: r.nextConsumer,
		logger:   r.logger,
		// Pass the shared yangParser so all targets benefit from level-1 YANG resolution.
		parser: newGNMIParser(r.logger, r.yangParser),
	}

	redialDelay := target.Redial
	if redialDelay <= 0 {
		redialDelay = 10 * time.Second
	}

	client.start(ctx, redialDelay)
}
