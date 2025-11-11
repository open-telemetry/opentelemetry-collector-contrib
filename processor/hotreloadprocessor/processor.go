// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/otelcol"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type loader[T any, P component.Component] interface {
	load(ctx context.Context,
		config otelcol.Config,
		settings otelprocessor.Settings,
		host component.Host,
		nextConsumer T) ([]P, error)
}

type HotReloadProcessor[T any, P component.Component] struct {
	logger              *zap.Logger
	telemetry           *hotreloadProcessorTelemetry
	telemetrySettings   component.TelemetrySettings
	done                chan struct{}
	shutdownOnce        sync.Once
	started             bool
	terminating         bool
	config              Config
	subprocessors       atomic.Pointer[[]P]
	firstSubprocessor   atomic.Pointer[P]
	host                component.Host
	set                 otelprocessor.Settings
	nextConsumer        T
	loader              loader[T, P]
	refreshConfig       *time.Ticker
	refreshInterval     time.Duration
	pendingShutdown     []P
	pendingShutdownMu   sync.Mutex
	shutdownTimer       *time.Timer
	shutdownDelay       time.Duration
	lastKnownKey        *string
	lastKnownConfigHash *string
	fileWatcher         *FileWatcher
}

func newHotReloadProcessor[T any, P component.Component](
	_ context.Context,
	set otelprocessor.Settings,
	cfg *Config,
	nextConsumer T,
	loader loader[T, P],
) (*HotReloadProcessor[T, P], error) {
	hp := &HotReloadProcessor[T, P]{
		logger: set.Logger,
		done:   make(chan struct{}),
		config: *cfg,
	}

	ht, err := newHotReloadProcessorTelemetry(set, cfg.ConfigurationPrefix)
	if err != nil {
		return nil, fmt.Errorf("error creating hotreload processor telemetry: %w", err)
	}
	hp.telemetry = ht
	hp.telemetrySettings = set.TelemetrySettings
	hp.set = set
	hp.nextConsumer = nextConsumer
	hp.loader = loader
	hp.refreshInterval = cfg.RefreshInterval
	hp.shutdownDelay = cfg.ShutdownDelay
	return hp, nil
}

func (hp *HotReloadProcessor[T, P]) applyConfigWithTelemetry(
	ctx context.Context,
	config otelcol.Config,
	key string,
	reload bool,
) error {
	newKey := false
	if hp.lastKnownKey == nil || *hp.lastKnownKey < key {
		hp.lastKnownKey = &key
		configHash, err := hp.hashConfig(config)
		if err != nil {
			return err
		}
		hp.lastKnownConfigHash = &configHash
		newKey = true
	}

	startTime := time.Now()
	err := hp.applyConfig(ctx, config, key, reload)
	duration := time.Since(startTime).Milliseconds()
	hp.telemetry.record(triggerReloadDuration, float64(duration), attribute.String("key", key))
	unixTimeInSeconds := float64(time.Now().Unix())
	if err != nil {
		if newKey {
			hp.telemetry.record(triggerNewestFileFailedTimestamp, unixTimeInSeconds,
				attribute.String("key", key),
				attribute.String("config_hash", *hp.lastKnownConfigHash),
				attribute.String("result", "failure"),
				attribute.String("reason", err.Error()),
				attribute.Bool("reload", reload),
			)
		}
	} else {
		if newKey {
			hp.telemetry.record(triggerNewestFileSuccessTimestamp, unixTimeInSeconds,
				attribute.String("key", key),
				attribute.String("config_hash", *hp.lastKnownConfigHash),
				attribute.String("result", "success"),
				attribute.Bool("reload", reload),
			)
		} else {
			hp.telemetry.record(triggerRollbackFileSuccessTimestamp, unixTimeInSeconds,
				attribute.String("key", key),
				attribute.String("config_hash", *hp.lastKnownConfigHash),
				attribute.String("result", "success"),
				attribute.Bool("reload", reload),
			)
		}
		hp.telemetry.record(triggerRunningProcessorsCount,
			float64(len(*hp.subprocessors.Load())),
		)
	}
	return err
}

func (hp *HotReloadProcessor[T, P]) applyConfig(
	ctx context.Context,
	config otelcol.Config,
	_ string,
	reload bool,
) error {
	subprocessors, err := hp.loader.load(ctx, config, hp.set, hp.host, hp.nextConsumer)
	if err != nil {
		return fmt.Errorf("failed to load subprocessors: %w", err)
	}

	for i := len(subprocessors) - 1; i >= 0; i-- {
		if err := subprocessors[i].Start(ctx, hp.host); err != nil {
			// call shutdown on all subprocessors
			for j := len(subprocessors) - 1; j >= i+1; j-- {
				err = subprocessors[j].Shutdown(ctx)
				if err != nil {
					hp.logger.Error("failed to shutdown subprocessor", zap.Error(err))
				}
			}
			return fmt.Errorf("failed to start subprocessor(%d): %w", i, err)
		}
	}

	oldSubprocessorsPtr := hp.subprocessors.Swap(&subprocessors)
	if len(subprocessors) > 0 {
		hp.firstSubprocessor.Store(&subprocessors[0])
	} else {
		hp.firstSubprocessor.Store(nil)
	}

	if reload {
		hp.logger.Debug("Reloading HotReloadProcessor processor")
		if oldSubprocessorsPtr != nil && len(*oldSubprocessorsPtr) > 0 {
			hp.pendingShutdownMu.Lock()
			hp.pendingShutdown = append(hp.pendingShutdown, (*oldSubprocessorsPtr)...)
			if hp.shutdownTimer != nil {
				if hp.shutdownTimer.Stop() {
					// drain if needed
					select {
					case <-hp.shutdownTimer.C:
					default:
					}
				}
			}
			hp.shutdownTimer = time.AfterFunc(hp.shutdownDelay, func() {
				hp.pendingShutdownMu.Lock()
				defer hp.pendingShutdownMu.Unlock()

				toShutdown := hp.pendingShutdown
				hp.pendingShutdown = nil

				for _, sp := range toShutdown {
					if err := sp.Shutdown(context.Background()); err != nil {
						hp.logger.Error("failed to shutdown subprocessor (delayed)", zap.Error(err))
					}
				}
			})
			hp.pendingShutdownMu.Unlock()
		}
	}

	return nil
}

// Start tells the component to start.
func (hp *HotReloadProcessor[T, P]) Start(ctx context.Context, host component.Host) error {
	hp.host = host
	client, err := createS3Client(ctx, hp.config.Region)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	hp.telemetry.record(
		triggerScan,
		float64(1),
		attribute.String("prefix", hp.config.ConfigurationPrefix),
	)
	hp.telemetry.record(
		triggerConfigRefreshInterval,
		float64(hp.refreshInterval.Milliseconds()),
	)
	hp.telemetry.record(
		triggerConfigShutdownDelay,
		float64(hp.shutdownDelay.Milliseconds()),
	)

	s3 := newS3Helper(
		hp.config,
		hp.logger,
		client,
		func(client S3Client, bucket string, key string) ListObjectsV2Paginator {
			delimiter := "/"
			return s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
				Bucket:    &bucket,
				Prefix:    &key,
				Delimiter: &delimiter,
			})
		},
	)
	err = s3.iterateConfigs(
		ctx,
		func(ctx context.Context, config otelcol.Config, key string) error {
			return hp.applyConfigWithTelemetry(ctx, config, key, false)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to get initial config: %w", err)
	}

	hp.logger.Info("Starting HotReloadProcessor processor")
	hp.refreshConfig = time.NewTicker(hp.refreshInterval)

	if hp.config.WatchPath != nil {
		hp.fileWatcher, err = NewFileWatcher(
			hp.logger,
			*hp.config.WatchPath,
			func(filePath string) error {
				hp.telemetry.record(
					triggerScan,
					float64(1),
					attribute.String("prefix", hp.config.ConfigurationPrefix),
				)
				err = s3.iterateConfigs(
					ctx,
					func(ctx context.Context, config otelcol.Config, key string) error {
						return hp.applyConfigWithTelemetry(ctx, config, key, true)
					},
				)
				if err != nil {
					hp.logger.Error("Failed to refresh config", zap.Error(err))
				}
				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("failed to create file watcher: %w", err)
		}

		err = hp.fileWatcher.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start file watcher: %w", err)
		}
	}

	go func() {
		for {
			select {
			case <-hp.done:
				hp.logger.Debug("Stream context done, stopping timer goroutine")
				return
			case <-hp.refreshConfig.C:
				hp.telemetry.record(
					triggerScan,
					float64(1),
					attribute.String("prefix", hp.config.ConfigurationPrefix),
				)
				err = s3.iterateConfigs(
					ctx,
					func(ctx context.Context, config otelcol.Config, key string) error {
						return hp.applyConfigWithTelemetry(ctx, config, key, true)
					},
				)
				if err != nil {
					hp.logger.Error("Failed to refresh config", zap.Error(err))
				}
			}
		}
	}()

	hp.started = true

	return nil
}

// Shutdown is invoked during service shutdown.
func (hp *HotReloadProcessor[T, P]) Shutdown(ctx context.Context) error {
	hp.pendingShutdownMu.Lock()
	defer hp.pendingShutdownMu.Unlock()

	hp.terminating = true

	// Shutdown all current subprocessors
	currentSubprocessorsPtr := hp.subprocessors.Swap(nil)
	if currentSubprocessorsPtr != nil {
		for i, subprocessor := range *currentSubprocessorsPtr {
			if err := subprocessor.Shutdown(ctx); err != nil {
				return fmt.Errorf("failed to shutdown subprocessor(%d): %w", i, err)
			}
		}
	}

	// Shutdown all pending subprocessors and cancel timer
	if hp.shutdownTimer != nil {
		if hp.shutdownTimer.Stop() {
			toShutdown := hp.pendingShutdown
			hp.pendingShutdown = nil
			for _, sp := range toShutdown {
				if err := sp.Shutdown(ctx); err != nil {
					hp.logger.Error("failed to shutdown subprocessor (pending)", zap.Error(err))
				}
			}
		} else {
			hp.logger.Error("failed to stop shutdown timer")
		}
	}
	hp.shutdownTimer = nil

	hp.shutdownOnce.Do(func() {
		if hp.started {
			hp.logger.Info("Stopping HotReloadProcessor processor")
			hp.refreshConfig.Stop()
			close(hp.done)
			if hp.fileWatcher != nil {
				err := hp.fileWatcher.Stop(ctx)
				if err != nil {
					hp.logger.Error("failed to stop file watcher", zap.Error(err))
				}
			}
			hp.started = false
		}
	})
	return nil
}

func (hp *HotReloadProcessor[T, P]) Capabilities() consumer.Capabilities {
	return processorCapabilities
}

func (hp *HotReloadProcessor[T, P]) consume(consumer func() error) error {
	// No lock needed, just use atomic loads
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime).Milliseconds()
		hp.telemetry.record(
			triggerProcessDuration,
			float64(duration),
		)
	}()
	return consumer()
}

func (hp *HotReloadProcessor[T, P]) hashConfig(config otelcol.Config) (string, error) {
	yaml, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(yaml)), nil
}
