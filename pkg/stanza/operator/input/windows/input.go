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
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type noopWindowsInputTelemetry struct{}

func (noopWindowsInputTelemetry) RecordEventSize(_ context.Context, _ string, _ int)      {}
func (noopWindowsInputTelemetry) RecordChannelSize(_ context.Context, _ string, _ int64)  {}
func (noopWindowsInputTelemetry) RecordMissedEvents(_ context.Context, _ string, _ int64) {}
func (noopWindowsInputTelemetry) RecordBatchSize(_ context.Context, _ string, _ int64)    {}

// Input is an operator that creates entries using the windows event log api.
type Input struct {
	helper.InputOperator
	bookmark                 Bookmark
	buffer                   *Buffer
	channel                  string
	ignoreChannelErrors      bool
	query                    *string
	maxReads                 int
	currentMaxReads          int
	startAt                  string
	raw                      bool
	eventDataFormat          EventDataFormat
	includeLogRecordOriginal bool
	excludeProviders         map[string]struct{}
	pollInterval             time.Duration
	waitTimeout              time.Duration
	// cancelEvent is a manual-reset Windows event handle signaled by Stop() to unblock
	// WaitForMultipleObjects in awaitAndReadEvents. A plain context cancellation cannot
	// interrupt a blocking Windows syscall, so this handle bridges Go's cancellation model
	// to the Windows API layer.
	cancelEvent           windows.Handle
	persister             operator.Persister
	publisherCache        publisherCache
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	subscription          Subscription
	maxEventsPerPollCycle int
	eventsReadInPollCycle int
	remote                RemoteConfig
	remoteSessionHandle   windows.Handle
	startRemoteSession    func() error
	processEvent          func(context.Context, Event) error
	telemetry             WindowsInputTelemetry
	logHandle             uintptr
	lastRecordID          uint64
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
	input.telemetry = noopWindowsInputTelemetry{}
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
	if i.remote.Domain != "" {
		login.Domain = windows.StringToUTF16Ptr(i.remote.Domain)
	}

	sessionHandle, err := evtOpenSession(EvtRPCLoginClass, &login, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open session for server %s: %w", i.remote.Server, err)
	}
	i.remoteSessionHandle = sessionHandle
	return nil
}

// resubscribe closes the current subscription and reopens it, tearing down and
// recreating the remote session as well for remote connections.
func (i *Input) resubscribe() error {
	if err := i.subscription.Close(); err != nil {
		return fmt.Errorf("failed to close subscription: %w", err)
	}

	if i.isRemote() {
		if err := i.stopRemoteSession(); err != nil {
			return fmt.Errorf("failed to stop remote session: %w", err)
		}
		i.subscription = NewRemoteSubscription(i.remote.Server, i.Logger())
		if err := i.startRemoteSession(); err != nil {
			return fmt.Errorf("failed to re-establish remote session for %s: %w", i.remote.Server, err)
		}
	}

	if err := i.subscription.Open(i.startAt, uintptr(i.remoteSessionHandle), i.channel, i.query, i.bookmark); err != nil {
		return fmt.Errorf("failed to reopen subscription: %w", err)
	}
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

	if i.channel != "" {
		// evtOpenLog opens a separate handle to the channel used exclusively for
		// querying metadata (e.g. record count) via EvtGetLogInfo. This is not the
		// subscription handle and does not affect event delivery. If this fails,
		// the otelcol_receiver_windows_event_log_channel_size metric will not be
		// recorded for the lifetime of this receiver instance.
		if handle, err := evtOpenLog(uintptr(i.remoteSessionHandle), windows.StringToUTF16Ptr(i.channel), EvtOpenChannelPath); err != nil {
			i.Logger().Warn("Failed to open log handle; otelcol_receiver_windows_event_log_channel_size metric will not be recorded",
				zap.String("channel", i.channel),
				zap.Error(err))
		} else {
			i.logHandle = handle
		}
	}

	i.bookmark = NewBookmark()
	offsetXML, err := i.getBookmarkOffset(ctx)
	if err != nil {
		_ = i.persister.Delete(ctx, i.getPersistKey())
	}

	if offsetXML != "" {
		if err := i.bookmark.Open(offsetXML); err != nil {
			return fmt.Errorf("failed to open bookmark: %w", err)
		}
	}

	i.publisherCache = newPublisherCache()

	subscriptionError := false
	subscription := NewLocalSubscription(i.Logger())
	if i.isRemote() {
		subscription = NewRemoteSubscription(i.remote.Server, i.Logger())
	}

	if err := subscription.Open(i.startAt, uintptr(i.remoteSessionHandle), i.channel, i.query, i.bookmark); err != nil {
		var errorString string
		if isNonTransientError(err) {
			if i.isRemote() {
				errorString = fmt.Sprintf("failed to open subscription for remote server: %s", i.remote.Server)
			} else {
				errorString = "failed to open local subscription"
			}
			if !i.ignoreChannelErrors {
				return fmt.Errorf("%s, error: %w", errorString, err)
			}
			subscriptionError = true
			i.Logger().Warn(errorString, zap.Error(err))
		} else {
			if i.isRemote() {
				i.Logger().Warn("Transient error opening subscription for remote server, continuing", zap.String("server", i.remote.Server), zap.Error(err))
			} else {
				i.Logger().Warn("Transient error opening local subscription, continuing", zap.Error(err))
			}
		}
	}

	if !subscriptionError {
		i.subscription = subscription
		if metadata.StanzaWindowsEventDrivenScrapingFeatureGate.IsEnabled() {
			cancelEvent, err := windows.CreateEvent(nil, 1, 0, nil) // manual-reset, initially non-signaled
			if err != nil {
				return fmt.Errorf("failed to create cancel event: %w", err)
			}
			i.cancelEvent = cancelEvent
			i.wg.Add(1)
			go i.awaitAndReadEvents(ctx)
		} else {
			i.wg.Add(1)
			go i.pollAndRead(ctx)
		}
	}

	return nil
}

// Stop will stop reading events from a subscription.
func (i *Input) Stop() error {
	// Warning: all calls made below must be safe to be done even if Start() was not called or failed.

	if i.cancel != nil {
		i.cancel()
	}

	if i.cancelEvent != 0 {
		// If this fails, wg.Wait() below will block forever since awaitAndReadEvents will never
		// return from WaitForMultipleObjects. Log loudly and continue.
		if err := windows.SetEvent(i.cancelEvent); err != nil {
			i.Logger().Error("Failed to signal cancel event during stop; shutdown may hang", zap.Error(err))
		}
	}

	i.wg.Wait()

	var errs error
	if i.cancelEvent != 0 {
		if err := windows.CloseHandle(i.cancelEvent); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to close cancel event: %w", err))
		}
		i.cancelEvent = 0
	}

	if err := i.subscription.Close(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to close subscription: %w", err))
	}

	if err := i.bookmark.Close(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to close bookmark: %w", err))
	}

	if err := i.publisherCache.evictAll(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to close publishers: %w", err))
	}

	if i.logHandle != 0 {
		if err := evtClose(i.logHandle); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to close log handle for %q: %w", i.channel, err))
		}
		i.logHandle = 0
	}

	return multierr.Append(errs, i.stopRemoteSession())
}

func (i *Input) pollAndRead(ctx context.Context) {
	defer i.wg.Done()

	for {
		i.eventsReadInPollCycle = 0

		select {
		case <-ctx.Done():
			return
		case <-time.After(i.pollInterval):
			if i.channel != "" && i.logHandle != 0 {
				var variant evtVariant
				var bufferUsed uint32
				if err := evtGetLogInfo(i.logHandle, EvtLogNumberOfLogRecords, &variant, &bufferUsed); err == nil {
					i.telemetry.RecordChannelSize(ctx, i.channel, int64(variant.Value))
				}
			}
			i.read(ctx)
		}
	}
}

func (i *Input) read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if !i.readBatch(ctx) {
				return
			}
		}
	}
}

// readBatch will read events from the subscription
func (i *Input) readBatch(ctx context.Context) bool {
	maxBatchSize := i.getCurrentBatchSize()
	if maxBatchSize <= 0 {
		return false
	}

	events, actualMaxReads, err := i.subscription.Read(maxBatchSize)

	// Update the current max reads if it changed
	if err == nil && actualMaxReads < maxBatchSize {
		i.currentMaxReads = actualMaxReads
		i.Logger().Debug("Encountered RPC_S_INVALID_BOUND, reduced batch size", zap.Int("current_batch_size", i.currentMaxReads), zap.Int("original_batch_size", i.maxReads))
	}

	if errors.Is(err, ErrorEVTQueryResultStale) {
		i.Logger().Warn("Windows Event Log bookmark invalidated: ring buffer overflowed and events were dropped; resubscribing",
			zap.String("channel", i.channel),
			zap.Uint64("last_record_id", i.lastRecordID),
		)
		i.lastRecordID = 0 // reset: next event establishes a new baseline to avoid a false gap warning
		if resubErr := i.resubscribe(); resubErr != nil {
			i.Logger().Error("Failed to resubscribe after ring-buffer overflow", zap.Error(resubErr))
		}
		return false
	}

	if err != nil {
		i.Logger().Error("Failed to read events from subscription", zap.Error(err))
		if i.isRemote() && (errors.Is(err, windows.ERROR_INVALID_HANDLE) || errors.Is(err, errSubscriptionHandleNotOpen)) {
			if resubErr := i.resubscribe(); resubErr != nil {
				i.Logger().Error("Failed to resubscribe after connection error", zap.Error(resubErr))
			}
		}
		return false
	}

	for n, event := range events {
		if err := i.processEvent(ctx, event); err != nil {
			i.Logger().Error("process event", zap.Error(err))
		}
		if len(events) == n+1 {
			i.updateBookmarkOffset(ctx, event)
			if err := i.subscription.bookmark.Update(event); err != nil {
				i.Logger().Error("Failed to update bookmark from event", zap.Error(err))
			}
		}
		event.Close()
	}

	i.eventsReadInPollCycle += len(events)
	if len(events) > 0 {
		i.telemetry.RecordBatchSize(ctx, i.channel, int64(len(events)))
	}
	return len(events) != 0
}

// awaitAndReadEvents is the event-driven alternative to pollAndRead. Instead of sleeping
// for a fixed interval it blocks on a Windows wait object that is signaled by the subscription
// when new events arrive. This reduces latency and avoids unnecessary wakeups.
func (i *Input) awaitAndReadEvents(ctx context.Context) {
	defer i.wg.Done()

	timeoutMs := uint32(i.waitTimeout.Milliseconds())
	for {
		ready, err := i.subscription.Wait(i.cancelEvent, timeoutMs)
		if err != nil {
			i.Logger().Error("Failed to wait for subscription signal", zap.Error(err))
			return
		}
		if !ready {
			// cancel event was signaled
			return
		}

		i.eventsReadInPollCycle = 0
		if i.channel != "" && i.logHandle != 0 {
			var variant evtVariant
			var bufferUsed uint32
			if err := evtGetLogInfo(i.logHandle, EvtLogNumberOfLogRecords, &variant, &bufferUsed); err == nil {
				i.telemetry.RecordChannelSize(ctx, i.channel, int64(variant.Value))
			}
		}
		i.read(ctx)
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

// checkRecordIDGap logs a warning when consecutive event RecordIDs are not
// contiguous, indicating that events were silently dropped from the ring buffer.
// Skipped in query mode (channel == "") because events from multiple channels
// have unrelated RecordID sequences.
func (i *Input) checkRecordIDGap(ctx context.Context, event parsedEvent) {
	if i.channel == "" {
		return
	}
	recordID := event.getRecordID()
	if recordID == 0 {
		return // absent or unparseable RecordID
	}
	if i.lastRecordID != 0 && recordID > i.lastRecordID+1 {
		missed := recordID - i.lastRecordID - 1
		i.Logger().Warn("Windows Event Log gap detected; events may have been missed from the channel",
			zap.String("channel", i.channel),
			zap.Uint64("last_record_id", i.lastRecordID),
			zap.Uint64("current_record_id", recordID),
			zap.Uint64("estimated_missed", missed),
		)
		i.telemetry.RecordMissedEvents(ctx, i.channel, int64(missed))
	}
	i.lastRecordID = recordID
}

func (i *Input) renderSimpleAndSend(ctx context.Context, event Event) error {
	render := event.RenderSimple
	if i.raw {
		render = event.RenderSimpleRaw
	}
	simpleEvent, err := render(i.buffer)
	if err != nil {
		return fmt.Errorf("render simple event: %w", err)
	}
	i.checkRecordIDGap(ctx, simpleEvent)
	i.telemetry.RecordEventSize(ctx, i.channel, len(simpleEvent.getOriginal()))
	return i.sendEvent(ctx, simpleEvent)
}

func (i *Input) renderDeepAndSend(ctx context.Context, event Event, publisher Publisher) error {
	render := event.RenderDeep
	if i.raw {
		render = event.RenderDeepRaw
	}
	deepEvent, err := render(i.buffer, publisher)
	if err == nil {
		i.checkRecordIDGap(ctx, deepEvent)
		i.telemetry.RecordEventSize(ctx, i.channel, len(deepEvent.getOriginal()))
		return i.sendEvent(ctx, deepEvent)
	}
	return multierr.Append(
		fmt.Errorf("render deep event: %w", err),
		i.renderSimpleAndSend(ctx, event),
	)
}

// processEvent will process and send an event retrieved from windows event log.
func (i *Input) processEventWithoutRenderingInfo(ctx context.Context, event Event) error {
	if len(i.excludeProviders) == 0 {
		return i.renderSimpleAndSend(ctx, event)
	}
	if _, exclude := i.getPublisherName(event); exclude {
		return nil
	}
	return i.renderSimpleAndSend(ctx, event)
}

func (i *Input) processEventWithRenderingInfo(ctx context.Context, event Event) error {
	providerName, exclude := i.getPublisherName(event)
	if exclude {
		return nil
	}

	publisher, err := i.publisherCache.get(providerName)
	if err != nil {
		return multierr.Append(
			fmt.Errorf("open event source for provider %q: %w", providerName, err),
			i.renderSimpleAndSend(ctx, event),
		)
	}

	if publisher.Valid() {
		return i.renderDeepAndSend(ctx, event, publisher)
	}
	return i.renderSimpleAndSend(ctx, event)
}

// sendEvent will send a parsedEvent as an entry to the operator's output.
//
// raw=true path: only event.getOriginal(), event.getSystemTime(), event.getLevel(),
// and event.getRenderedLevel() are called. If you add a field access here that
// runs when raw=true, add a corresponding method to parsedEvent and rawParsedEvent.
func (i *Input) sendEvent(ctx context.Context, event parsedEvent) error {
	var body any = event.getOriginal()
	if !i.raw {
		body = formattedBody(event.toEventXML(), i.eventDataFormat)
	}

	e, err := i.NewEntry(body)
	if err != nil {
		return fmt.Errorf("create entry: %w", err)
	}

	e.Timestamp = parseTimestamp(event.getSystemTime())
	e.Severity = parseSeverity(event.getRenderedLevel(), event.getLevel())

	if i.remote.Server != "" {
		e.AddAttribute("server.address", i.remote.Server)
	}

	if i.includeLogRecordOriginal {
		e.AddAttribute(string(conventions.LogRecordOriginalKey), event.getOriginal())
	}

	return i.Write(ctx, e)
}

// getBookmarkXML will get the bookmark xml from the offsets database.
func (i *Input) getBookmarkOffset(ctx context.Context) (string, error) {
	bytes, err := i.persister.Get(ctx, i.getPersistKey())
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

	if err := i.persister.Set(ctx, i.getPersistKey(), []byte(bookmarkXML)); err != nil {
		i.Logger().Error("failed to set offsets", zap.Error(err))
		return
	}
}

func (i *Input) getPersistKey() string {
	if i.query != nil {
		return *i.query
	}

	return i.channel
}

func (i *Input) getCurrentBatchSize() int {
	if i.maxEventsPerPollCycle == 0 {
		return i.currentMaxReads
	}

	return min(i.currentMaxReads, i.maxEventsPerPollCycle-i.eventsReadInPollCycle)
}
