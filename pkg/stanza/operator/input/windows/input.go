// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

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
	publisherCache   publisherCache
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// Start will start reading events from a subscription.
func (i *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	i.persister = persister

	i.bookmark = NewBookmark()
	offsetXML, err := i.getBookmarkOffset(ctx)
	if err != nil {
		i.Logger().Error("Failed to open bookmark, continuing without previous bookmark", zap.Error(err))
		_ = i.persister.Delete(ctx, i.channel)
	}

	if offsetXML != "" {
		if err := i.bookmark.Open(offsetXML); err != nil {
			return fmt.Errorf("failed to open bookmark: %w", err)
		}
	}

	i.subscription = NewSubscription()
	if err := i.subscription.Open(i.channel, i.startAt, i.bookmark); err != nil {
		return fmt.Errorf("failed to open subscription: %w", err)
	}

	i.publisherCache = newPublisherCache()

	i.wg.Add(1)
	go i.readOnInterval(ctx)
	return nil
}

// Stop will stop reading events from a subscription.
func (i *Input) Stop() error {
	i.cancel()
	i.wg.Wait()

	if err := i.subscription.Close(); err != nil {
		return fmt.Errorf("failed to close subscription: %w", err)
	}

	if err := i.bookmark.Close(); err != nil {
		return fmt.Errorf("failed to close bookmark: %w", err)
	}

	if err := i.publisherCache.evictAll(); err != nil {
		return fmt.Errorf("failed to close publishers: %w", err)
	}

	return nil
}

// readOnInterval will read events with respect to the polling interval.
func (i *Input) readOnInterval(ctx context.Context) {
	defer i.wg.Done()

	ticker := time.NewTicker(i.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.readToEnd(ctx)
		}
	}
}

// readToEnd will read events from the subscription until it reaches the end of the channel.
func (i *Input) readToEnd(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if count := i.read(ctx); count == 0 {
				return
			}
		}
	}
}

// read will read events from the subscription.
func (i *Input) read(ctx context.Context) int {
	events, err := i.subscription.Read(i.maxReads)
	if err != nil {
		i.Logger().Error("Failed to read events from subscription", zap.Error(err))
		return 0
	}

	for n, event := range events {
		i.processEvent(ctx, event)
		if len(events) == n+1 {
			i.updateBookmarkOffset(ctx, event)
		}
		event.Close()
	}

	return len(events)
}

// processEvent will process and send an event retrieved from windows event log.
func (i *Input) processEvent(ctx context.Context, event Event) {
	if i.raw {
		if len(i.excludeProviders) > 0 {
			simpleEvent, err := event.RenderSimple(i.buffer)
			if err != nil {
				i.Logger().Error("Failed to render simple event", zap.Error(err))
				return
			}

			for _, excludeProvider := range i.excludeProviders {
				if simpleEvent.Provider.Name == excludeProvider {
					return
				}
			}
		}

		rawEvent, err := event.RenderRaw(i.buffer)
		if err != nil {
			i.Logger().Error("Failed to render raw event", zap.Error(err))
			return
		}
		i.sendEventRaw(ctx, rawEvent)
		return
	}
	simpleEvent, err := event.RenderSimple(i.buffer)
	if err != nil {
		i.Logger().Error("Failed to render simple event", zap.Error(err))
		return
	}

	for _, excludeProvider := range i.excludeProviders {
		if simpleEvent.Provider.Name == excludeProvider {
			return
		}
	}

	publisher, openPublisherErr := i.publisherCache.get(simpleEvent.Provider.Name)
	if openPublisherErr != nil {
		i.Logger().Warn(
			"Failed to open event source, respective log entries cannot be formatted",
			zap.String("provider", simpleEvent.Provider.Name), zap.Error(openPublisherErr))
	}

	if !publisher.Valid() {
		i.sendEvent(ctx, simpleEvent)
		return
	}

	formattedEvent, err := event.RenderFormatted(i.buffer, publisher)
	if err != nil {
		i.Logger().Error("Failed to render formatted event", zap.Error(err))
		i.sendEvent(ctx, simpleEvent)
		return
	}

	i.sendEvent(ctx, formattedEvent)
}

// sendEvent will send EventXML as an entry to the operator's output.
func (i *Input) sendEvent(ctx context.Context, eventXML EventXML) {
	body := eventXML.parseBody()
	entry, err := i.NewEntry(body)
	if err != nil {
		i.Logger().Error("Failed to create entry", zap.Error(err))
		return
	}

	entry.Timestamp = eventXML.parseTimestamp()
	entry.Severity = eventXML.parseRenderedSeverity()
	i.Write(ctx, entry)
}

func (i *Input) sendEventRaw(ctx context.Context, eventRaw EventRaw) {
	body := eventRaw.parseBody()
	entry, err := i.NewEntry(body)
	if err != nil {
		i.Logger().Error("Failed to create entry", zap.Error(err))
		return
	}

	entry.Timestamp = eventRaw.parseTimestamp()
	entry.Severity = eventRaw.parseRenderedSeverity()
	i.Write(ctx, entry)
}

// getBookmarkXML will get the bookmark xml from the offsets database.
func (i *Input) getBookmarkOffset(ctx context.Context) (string, error) {
	bytes, err := i.persister.Get(ctx, i.channel)
	return string(bytes), err
}

// updateBookmark will update the bookmark xml and save it in the offsets database.
func (i *Input) updateBookmarkOffset(ctx context.Context, event Event) {
	if err := i.bookmark.Update(event); err != nil {
		i.Logger().Error("Failed to update bookmark from event", zap.Error(err))
		return
	}

	bookmarkXML, err := i.bookmark.Render(i.buffer)
	if err != nil {
		i.Logger().Error("Failed to render bookmark xml", zap.Error(err))
		return
	}

	if err := i.persister.Set(ctx, i.channel, []byte(bookmarkXML)); err != nil {
		i.Logger().Error("failed to set offsets", zap.Error(err))
		return
	}
}
