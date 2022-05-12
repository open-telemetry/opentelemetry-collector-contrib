// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-log-collection/operator/input/windows"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("windows_eventlog_input", func() operator.Builder { return NewDefaultConfig() })
}

// EventLogConfig is the configuration of a windows event log operator.
type EventLogConfig struct {
	helper.InputConfig `mapstructure:",squash" yaml:",inline"`
	Channel            string          `mapstructure:"channel" json:"channel" yaml:"channel"`
	MaxReads           int             `mapstructure:"max_reads,omitempty" json:"max_reads,omitempty" yaml:"max_reads,omitempty"`
	StartAt            string          `mapstructure:"start_at,omitempty" json:"start_at,omitempty" yaml:"start_at,omitempty"`
	PollInterval       helper.Duration `mapstructure:"poll_interval,omitempty" json:"poll_interval,omitempty" yaml:"poll_interval,omitempty"`
}

// Build will build a windows event log operator.
func (c *EventLogConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
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

	return &EventLogInput{
		InputOperator: inputOperator,
		buffer:        NewBuffer(),
		channel:       c.Channel,
		maxReads:      c.MaxReads,
		startAt:       c.StartAt,
		pollInterval:  c.PollInterval,
	}, nil
}

// NewDefaultConfig will return an event log config with default values.
func NewDefaultConfig() *EventLogConfig {
	return &EventLogConfig{
		InputConfig: helper.NewInputConfig("", "windows_eventlog_input"),
		MaxReads:    100,
		StartAt:     "end",
		PollInterval: helper.Duration{
			Duration: 1 * time.Second,
		},
	}
}

// EventLogInput is an operator that creates entries using the windows event log api.
type EventLogInput struct {
	helper.InputOperator
	bookmark     Bookmark
	subscription Subscription
	buffer       Buffer
	channel      string
	maxReads     int
	startAt      string
	pollInterval helper.Duration
	persister    operator.Persister
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Start will start reading events from a subscription.
func (e *EventLogInput) Start(persister operator.Persister) error {
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
			return fmt.Errorf("failed to open bookmark: %s", err)
		}
	}

	e.subscription = NewSubscription()
	if err := e.subscription.Open(e.channel, e.startAt, e.bookmark); err != nil {
		return fmt.Errorf("failed to open subscription: %s", err)
	}

	e.wg.Add(1)
	go e.readOnInterval(ctx)
	return nil
}

// Stop will stop reading events from a subscription.
func (e *EventLogInput) Stop() error {
	e.cancel()
	e.wg.Wait()

	if err := e.subscription.Close(); err != nil {
		return fmt.Errorf("failed to close subscription: %s", err)
	}

	if err := e.bookmark.Close(); err != nil {
		return fmt.Errorf("failed to close bookmark: %s", err)
	}

	return nil
}

// readOnInterval will read events with respect to the polling interval.
func (e *EventLogInput) readOnInterval(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.pollInterval.Raw())
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
func (e *EventLogInput) readToEnd(ctx context.Context) {
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
func (e *EventLogInput) read(ctx context.Context) int {
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
func (e *EventLogInput) processEvent(ctx context.Context, event Event) {
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
func (e *EventLogInput) sendEvent(ctx context.Context, eventXML EventXML) {
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

// getBookmarkXML will get the bookmark xml from the offsets database.
func (e *EventLogInput) getBookmarkOffset(ctx context.Context) (string, error) {
	bytes, err := e.persister.Get(ctx, e.channel)
	return string(bytes), err
}

// updateBookmark will update the bookmark xml and save it in the offsets database.
func (e *EventLogInput) updateBookmarkOffset(ctx context.Context, event Event) {
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
