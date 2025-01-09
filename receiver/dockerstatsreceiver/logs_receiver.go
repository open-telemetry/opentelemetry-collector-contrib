// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

const dockerEventPrefix = "docker"

type logsReceiver struct {
	config   *Config
	settings receiver.Settings

	cancel      context.CancelFunc
	consumer    consumer.Logs
	eventPoller *dockerEventPoller
}

func newLogsReceiver(set receiver.Settings, config *Config, consumer consumer.Logs) *logsReceiver {
	return &logsReceiver{
		config:   config,
		settings: set,
		consumer: consumer,
	}
}

func (r *logsReceiver) Start(ctx context.Context, _ component.Host) error {
	var err error
	client, err := docker.NewDockerClient(&r.config.Config, r.settings.Logger)
	if err != nil {
		return err
	}

	if err = client.LoadContainerList(ctx); err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.eventPoller = newDockerEventPoller(r.config, client, r.settings.Logger, r.consumeDockerEvent)
	go r.eventPoller.Start(cctx)
	return nil
}

func getDockerBackoffConfig(config *Config) *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     config.MinDockerRetryWait,
		MaxInterval:         config.MaxDockerRetryWait,
		MaxElapsedTime:      0,
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}

// dockerEventPoller manages the lifecycle and event stream of the Docker daemon connection.
type dockerEventPoller struct {
	config       *Config
	client       *docker.Client
	logger       *zap.Logger
	eventHandler func(context.Context, *events.Message) error
	backoff      *backoff.ExponentialBackOff
	sync.WaitGroup
}

func newDockerEventPoller(
	config *Config,
	client *docker.Client,
	logger *zap.Logger,
	handler func(context.Context, *events.Message) error,
) *dockerEventPoller {
	return &dockerEventPoller{
		config:       config,
		client:       client,
		logger:       logger,
		eventHandler: handler,
		backoff:      getDockerBackoffConfig(config),
	}
}

func (d *dockerEventPoller) Start(ctx context.Context) {
	filterArgs := filters.Args{}
	if len(d.config.Logs.Filters) > 0 {
		filterArgs = filters.NewArgs()
		for k, v := range d.config.Logs.Filters {
			for _, elem := range v {
				filterArgs.Add(k, elem)
			}
		}
	}
	for {
		// event stream can be interrupted by async errors (connection or other).
		// client caller must retry to restart processing. retry with backoff here
		// except for context cancellation.
		eventChan, errChan := d.client.Events(ctx, events.ListOptions{
			Since:   d.config.Logs.Since,
			Until:   d.config.Logs.Until,
			Filters: filterArgs,
		})

		err := d.processEvents(ctx, eventChan, errChan)
		if errors.Is(err, context.Canceled) {
			return
		}
		// io.EOF is expected, no need to log but still needs retrying
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			d.logger.Error("Async error while processing docker events, reconnecting to docker daemon", zap.Error(err))
		}
		nextBackoff := d.backoff.NextBackOff()
		select {
		case <-ctx.Done():
			return
		case <-time.After(nextBackoff):
			continue
		}
	}
}

func (d *dockerEventPoller) processEvents(ctx context.Context, eventChan <-chan events.Message, errChan <-chan error) error {
	d.Add(1)
	defer d.Done()
	processedOnce := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-eventChan:
			// for the given method invocation, processing event indicates a successful daemon connection.
			// backoff should be reset, no need to do this afterwards since the connection is already established.
			if !processedOnce {
				d.backoff.Reset()
				processedOnce = true
			}
			if err := d.eventHandler(ctx, &event); err != nil {
				d.logger.Error("Failed to process docker event", zap.Error(err))
			}
		case err := <-errChan:
			return err
		}
	}
}

// TODO: add batching based on time/volume. for now, one event -> one logs
func (r *logsReceiver) consumeDockerEvent(ctx context.Context, event *events.Message) error {
	if event.Type == "" {
		return nil
	}
	logs := plog.NewLogs()
	logRecord := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(event.Time, event.TimeNano)))
	attrs := logRecord.Attributes()
	action := string(event.Action)
	// for cases with health_status: running etc., event.name should be
	// properly namespaced as docker.container.health_status.running
	if strings.Contains(action, ": ") {
		action = strings.Join(strings.Split(action, ": "), ".")
	}
	if action != "" {
		// i.e. docker.container.start
		attrs.PutStr("event.name", fmt.Sprintf("%s.%s.%s", dockerEventPrefix, event.Type, action))
	} else {
		attrs.PutStr("event.name", fmt.Sprintf("%s.%s", dockerEventPrefix, event.Type))
	}
	if event.Scope != "" {
		attrs.PutStr("event.scope", event.Scope)
	}
	if event.Actor.ID != "" {
		attrs.PutStr("event.id", event.Actor.ID)
	}
	// body exactly replicates actor attributes
	if len(event.Actor.Attributes) > 0 {
		actorAttrs := logRecord.Body().SetEmptyMap()
		for k, v := range event.Actor.Attributes {
			if k != "" {
				actorAttrs.PutStr(k, v)
			}
		}
	}
	return r.consumer.ConsumeLogs(ctx, logs)
}

func (r *logsReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.eventPoller != nil {
		r.eventPoller.Wait()
	}
	return nil
}
