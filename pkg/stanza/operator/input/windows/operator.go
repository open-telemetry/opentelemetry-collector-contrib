// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "windows_eventlog_input"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig will return an event log config with default values.
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfig will return an event log config with default values.
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig:  helper.NewInputConfig(operatorID, operatorType),
		MaxReads:     100,
		StartAt:      "end",
		PollInterval: 1 * time.Second,
	}
}

// Config is the configuration of a windows event log operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	Channel            string        `mapstructure:"channel"`
	MaxReads           int           `mapstructure:"max_reads,omitempty"`
	StartAt            string        `mapstructure:"start_at,omitempty"`
	PollInterval       time.Duration `mapstructure:"poll_interval,omitempty"`
	Raw                bool          `mapstructure:"raw,omitempty"`
	ExcludeProviders   []string      `mapstructure:"exclude_providers,omitempty"`
}

// Build will build a windows event log operator.
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.Channel == "" {
		return nil, fmt.Errorf("missing required `channel` field")
	}

	if c.MaxReads < 1 {
		return nil, fmt.Errorf("the `max_reads` field must be greater than zero")
	}

	if c.StartAt != "end" && c.StartAt != "beginning" {
		return nil, fmt.Errorf("the `start_at` field must be set to `beginning` or `end`")
	}

	return &Input{
		InputOperator:    inputOperator,
		buffer:           NewBuffer(),
		channel:          c.Channel,
		maxReads:         c.MaxReads,
		startAt:          c.StartAt,
		pollInterval:     c.PollInterval,
		raw:              c.Raw,
		excludeProviders: c.ExcludeProviders,
	}, nil
}

// Input is an operator that creates entries using the windows event log api.
type Input struct {
	helper.InputOperator
	bookmark         Bookmark
	subscription     Subscription
	buffer           Buffer
	channel          string
	maxReads         int
	startAt          string
	raw              bool
	excludeProviders []string
	pollInterval     time.Duration
	persister        operator.Persister
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// Start will start reading events from a subscription.
func (e *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.persister = persister

	e.bookmark = NewBookmark()
	offsetXML, err := e.getBookmarkOffset(ctx)
	if err != nil {
		e.Errorf("Failed to open bookmark, continuing without previous bookmark: %s", err)
		e.persister.Delete(ctx, e.channel)
	}

	if offsetXML != "" {
		if err := e.bookmark.Open(offsetXML); err != nil {
			return fmt.Errorf("failed to open bookmark: %w", err)
		}
	}

	e.subscription = NewSubscription()
	if err := e.subscription.Open(e.channel, e.startAt, e.bookmark); err != nil {
		return fmt.Errorf("failed to open subscription: %w", err)
	}

	e.wg.Add(1)
	go e.readOnInterval(ctx)
	return nil
}

// Stop will stop reading events from a subscription.
func (e *Input) Stop() error {
	e.cancel()
	e.wg.Wait()

	if err := e.subscription.Close(); err != nil {
		return fmt.Errorf("failed to close subscription: %w", err)
	}

	if err := e.bookmark.Close(); err != nil {
		return fmt.Errorf("failed to close bookmark: %w", err)
	}

	return nil
}

// readOnInterval will read events with respect to the polling interval.
func (e *Input) readOnInterval(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.readToEnd(ctx)
		}
	}
}

// readToEnd will read events from the subscription until it reaches the end of the channel.
func (e *Input) readToEnd(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if count := e.read(ctx); count == 0 {
				return
			}
		}
	}
}

// read will read events from the subscription.
func (e *Input) read(ctx context.Context) int {
	events, err := e.subscription.Read(e.maxReads)
	if err != nil {
		e.Errorf("Failed to read events from subscription: %s", err)
		return 0
	}

	for i, event := range events {
		e.processEvent(ctx, event)
		if len(events) == i+1 {
			e.updateBookmarkOffset(ctx, event)
		}
		event.Close()
	}

	return len(events)
}

// processEvent will process and send an event retrieved from windows event log.
func (e *Input) processEvent(ctx context.Context, event Event) {
	if len(e.excludeProviders) > 0 {
		simpleEvent, err := event.RenderSimple(e.buffer)
		if err != nil {
			e.Errorf("Failed to render simple event: %s", err)
			return
		}

		for _, excludeProvider := range e.excludeProviders {
			if simpleEvent.Provider.Name == excludeProvider {
				e.Debug("Stopped processing event with excluded provider ", excludeProvider)
				return
			}
		}
	}

	if e.raw {
		rawEvent, err := event.RenderRaw(e.buffer)
		if err != nil {
			e.Errorf("Failed to render raw event: %s", err)
			return
		}
		e.sendEventRaw(ctx, rawEvent)
		return
	}
	simpleEvent, err := event.RenderSimple(e.buffer)
	if err != nil {
		e.Errorf("Failed to render simple event: %s", err)
		return
	}

	publisher := NewPublisher()
	if err := publisher.Open(simpleEvent.Provider.Name); err != nil {
		e.Errorf("Failed to open publisher: %s: writing log entry to pipeline without metadata", err)
		e.sendEvent(ctx, simpleEvent)
		return
	}
	defer publisher.Close()

	formattedEvent, err := event.RenderFormatted(e.buffer, publisher)
	if err != nil {
		e.Errorf("Failed to render formatted event: %s", err)
		e.sendEvent(ctx, simpleEvent)
		return
	}

	e.sendEvent(ctx, formattedEvent)
}

// sendEvent will send EventXML as an entry to the operator's output.
func (e *Input) sendEvent(ctx context.Context, eventXML EventXML) {
	body := eventXML.parseBody()
	entry, err := e.NewEntry(body)
	if err != nil {
		e.Errorf("Failed to create entry: %s", err)
		return
	}

	entry.Timestamp = eventXML.parseTimestamp()
	entry.Severity = eventXML.parseRenderedSeverity()
	e.Write(ctx, entry)
}

func (e *Input) sendEventRaw(ctx context.Context, eventRaw EventRaw) {
	body := eventRaw.parseBody()
	entry, err := e.NewEntry(body)
	if err != nil {
		e.Errorf("Failed to create entry: %s", err)
		return
	}

	entry.Timestamp = eventRaw.parseTimestamp()
	entry.Severity = eventRaw.parseRenderedSeverity()
	e.Write(ctx, entry)
}

// getBookmarkXML will get the bookmark xml from the offsets database.
func (e *Input) getBookmarkOffset(ctx context.Context) (string, error) {
	bytes, err := e.persister.Get(ctx, e.channel)
	return string(bytes), err
}

// updateBookmark will update the bookmark xml and save it in the offsets database.
func (e *Input) updateBookmarkOffset(ctx context.Context, event Event) {
	if err := e.bookmark.Update(event); err != nil {
		e.Errorf("Failed to update bookmark from event: %s", err)
		return
	}

	bookmarkXML, err := e.bookmark.Render(e.buffer)
	if err != nil {
		e.Errorf("Failed to render bookmark xml: %s", err)
		return
	}

	if err := e.persister.Set(ctx, e.channel, []byte(bookmarkXML)); err != nil {
		e.Errorf("failed to set offsets: %s", err)
		return
	}
}
