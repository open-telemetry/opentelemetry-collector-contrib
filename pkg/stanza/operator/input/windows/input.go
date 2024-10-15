// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that creates entries using the windows event log api.
type Input struct {
	helper.InputOperator
	bookmark            Bookmark
	buffer              Buffer
	channel             string
	maxReads            int
	startAt             string
	raw                 bool
	excludeProviders    map[string]struct{}
	pollInterval        time.Duration
	persister           operator.Persister
	publisherCache      publisherCache
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	subscription        Subscription
	remote              RemoteConfig
	remoteSessionHandle windows.Handle
	startRemoteSession  func() error
	processEvent        func(context.Context, Event)
}

// newInput creates a new Input operator.
func newInput(settings component.TelemetrySettings) *Input {
	basicConfig := helper.NewBasicConfig("windowseventlog", "input")
	basicOperator, _ := basicConfig.Build(settings)

	input := &Input{
		InputOperator: helper.InputOperator{
			WriterOperator: helper.WriterOperator{
				BasicOperator: basicOperator,
			},
		},
	}
	input.startRemoteSession = input.defaultStartRemoteSession
	return input
}

// defaultStartRemoteSession starts a remote session for reading event logs from a remote server.
func (i *Input) defaultStartRemoteSession() error {
	if i.remote.Server == "" {
		return nil
	}

	login := EvtRPCLogin{
		Server:   windows.StringToUTF16Ptr(i.remote.Server),
		User:     windows.StringToUTF16Ptr(i.remote.Username),
		Password: windows.StringToUTF16Ptr(i.remote.Password),
	}

	sessionHandle, err := evtOpenSession(EvtRPCLoginClass, &login, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open session for server %s: %w", i.remote.Server, err)
	}
	i.remoteSessionHandle = sessionHandle
	return nil
}

// stopRemoteSession stops the remote session if it is active.
func (i *Input) stopRemoteSession() error {
	if i.remoteSessionHandle != 0 {
		if err := evtClose(uintptr(i.remoteSessionHandle)); err != nil {
			return fmt.Errorf("failed to close remote session handle for server %s: %w", i.remote.Server, err)
		}
		i.remoteSessionHandle = 0
	}
	return nil
}

// isRemote checks if the input is configured for remote access.
func (i *Input) isRemote() bool {
	return i.remote.Server != ""
}

// isNonTransientError checks if the error is likely non-transient.
func isNonTransientError(err error) bool {
	return errors.Is(err, windows.ERROR_EVT_CHANNEL_NOT_FOUND) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// Start will start reading events from a subscription.
func (i *Input) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel

	i.persister = persister

	if i.isRemote() {
		if err := i.startRemoteSession(); err != nil {
			return fmt.Errorf("failed to start remote session for server %s: %w", i.remote.Server, err)
		}
	}

	i.bookmark = NewBookmark()
	offsetXML, err := i.getBookmarkOffset(ctx)
	if err != nil {
		_ = i.persister.Delete(ctx, i.channel)
	}

	if offsetXML != "" {
		if err := i.bookmark.Open(offsetXML); err != nil {
			return fmt.Errorf("failed to open bookmark: %w", err)
		}
	}

	i.publisherCache = newPublisherCache()

	subscription := NewLocalSubscription()
	if i.isRemote() {
		subscription = NewRemoteSubscription(i.remote.Server)
	}

	if err := subscription.Open(i.startAt, uintptr(i.remoteSessionHandle), i.channel, i.bookmark); err != nil {
		if isNonTransientError(err) {
			if i.isRemote() {
				return fmt.Errorf("failed to open subscription for remote server %s: %w", i.remote.Server, err)
			}
			return fmt.Errorf("failed to open local subscription: %w", err)
		}
		if i.isRemote() {
			i.Logger().Warn("Transient error opening subscription for remote server, continuing", zap.String("server", i.remote.Server), zap.Error(err))
		} else {
			i.Logger().Warn("Transient error opening local subscription, continuing", zap.Error(err))
		}
	}

	i.subscription = subscription
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

	return i.stopRemoteSession()
}

// readOnInterval will read events with respect to the polling interval until it reaches the end of the channel.
func (i *Input) readOnInterval(ctx context.Context) {
	defer i.wg.Done()

	ticker := time.NewTicker(i.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.read(ctx)
		}
	}
}

// read will read events from the subscription.
func (i *Input) read(ctx context.Context) {
	events, err := i.subscription.Read(i.maxReads)
	if err != nil {
		i.Logger().Error("Failed to read events from subscription", zap.Error(err))
		if i.isRemote() && (errors.Is(err, windows.ERROR_INVALID_HANDLE) || errors.Is(err, errSubscriptionHandleNotOpen)) {
			i.Logger().Info("Resubscribing, closing remote subscription")
			closeErr := i.subscription.Close()
			if closeErr != nil {
				i.Logger().Error("Failed to close remote subscription", zap.Error(closeErr))
				return
			}
			if err := i.stopRemoteSession(); err != nil {
				i.Logger().Error("Failed to close remote session", zap.Error(err))
			}
			i.Logger().Info("Resubscribing, creating remote subscription")
			i.subscription = NewRemoteSubscription(i.remote.Server)
			if err := i.startRemoteSession(); err != nil {
				i.Logger().Error("Failed to re-establish remote session", zap.String("server", i.remote.Server), zap.Error(err))
				return
			}
			if err := i.subscription.Open(i.startAt, uintptr(i.remoteSessionHandle), i.channel, i.bookmark); err != nil {
				i.Logger().Error("Failed to re-open subscription for remote server", zap.String("server", i.remote.Server), zap.Error(err))
				return
			}
		}
		return
	}

	for n, event := range events {
		i.processEvent(ctx, event)
		if len(events) == n+1 {
			i.updateBookmarkOffset(ctx, event)
		}
		event.Close()
	}
}

func (i *Input) getPublisherName(event Event) (name string, excluded bool) {
	providerName, err := event.GetPublisherName(i.buffer)
	if err != nil {
		i.Logger().Error("Failed to get provider name", zap.Error(err))
		return "", true
	}
	if _, exclude := i.excludeProviders[providerName]; exclude {
		return "", true
	}

	return providerName, false
}

func (i *Input) renderSimpleAndSend(ctx context.Context, event Event) {
	simpleEvent, err := event.RenderSimple(i.buffer)
	if err != nil {
		i.Logger().Error("Failed to render simple event", zap.Error(err))
		return
	}
	i.sendEvent(ctx, simpleEvent)
}

func (i *Input) renderDeepAndSend(ctx context.Context, event Event, publisher Publisher) {
	deepEvent, err := event.RenderDeep(i.buffer, publisher)
	if err == nil {
		i.sendEvent(ctx, deepEvent)
		return
	}
	i.Logger().Error("Failed to render formatted event", zap.Error(err))
	i.renderSimpleAndSend(ctx, event)
}

// processEvent will process and send an event retrieved from windows event log.
func (i *Input) processEventWithoutRenderingInfo(ctx context.Context, event Event) {
	if len(i.excludeProviders) == 0 {
		i.renderSimpleAndSend(ctx, event)
		return
	}
	if _, exclude := i.getPublisherName(event); exclude {
		return
	}
	i.renderSimpleAndSend(ctx, event)
}

func (i *Input) processEventWithRenderingInfo(ctx context.Context, event Event) {
	providerName, exclude := i.getPublisherName(event)
	if exclude {
		return
	}

	publisher, err := i.publisherCache.get(providerName)
	if err != nil {
		i.Logger().Warn(
			"Failed to open event source, respective log entries cannot be formatted",
			zap.String("provider", providerName), zap.Error(err))
		i.renderSimpleAndSend(ctx, event)
		return
	}

	if publisher.Valid() {
		i.renderDeepAndSend(ctx, event, publisher)
		return
	}
	i.renderSimpleAndSend(ctx, event)
}

// sendEvent will send EventXML as an entry to the operator's output.
func (i *Input) sendEvent(ctx context.Context, eventXML *EventXML) {
	var body any = eventXML.Original
	if !i.raw {
		body = formattedBody(eventXML)
	}

	e, err := i.NewEntry(body)
	if err != nil {
		i.Logger().Error("Failed to create entry", zap.Error(err))
		return
	}

	e.Timestamp = parseTimestamp(eventXML.TimeCreated.SystemTime)
	e.Severity = parseSeverity(eventXML.RenderedLevel, eventXML.Level)

	if i.remote.Server != "" {
		e.Attributes["server.address"] = i.remote.Server
	}

	_ = i.Write(ctx, e)
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
