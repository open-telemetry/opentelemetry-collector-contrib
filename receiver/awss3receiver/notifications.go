// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opamp-go/client/types"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

const (
	IngestStatusCompleted   = "completed"
	IngestStatusFailed      = "failed"
	IngestStatusIngesting   = "ingesting"
	CustomCapability        = "org.opentelemetry.collector.receiver.awss3"
	maxNotificationAttempts = 3
)

type statusNotification struct {
	TelemetryType  string
	IngestStatus   string
	StartTime      time.Time
	EndTime        time.Time
	IngestTime     time.Time
	FailureMessage string
}

type statusNotifier interface {
	Start(ctx context.Context, host component.Host) error
	Shutdown(ctx context.Context) error
	SendStatus(ctx context.Context, message statusNotification)
}

type opampNotifier struct {
	logger           *zap.Logger
	opampExtensionID component.ID
	handler          opampcustommessages.CustomCapabilityHandler
}

func newNotifier(config *Config, logger *zap.Logger) statusNotifier {
	if config.Notifications.OpAMP != nil {
		return &opampNotifier{opampExtensionID: *config.Notifications.OpAMP, logger: logger}
	}
	return nil
}

func (n *opampNotifier) Start(_ context.Context, host component.Host) error {
	ext, ok := host.GetExtensions()[n.opampExtensionID]
	if !ok {
		return fmt.Errorf("extension %q does not exist", n.opampExtensionID)
	}

	registry, ok := ext.(opampcustommessages.CustomCapabilityRegistry)
	if !ok {
		return fmt.Errorf("extension %q is not a custom message registry", n.opampExtensionID)
	}

	handler, err := registry.Register(CustomCapability)
	if err != nil {
		return fmt.Errorf("failed to register custom capability: %w", err)
	}
	if handler == nil {
		return errors.New("custom capability handler is nil")
	}
	n.handler = handler
	return nil
}

func (n *opampNotifier) Shutdown(_ context.Context) error {
	if n.handler != nil {
		n.handler.Unregister()
	}
	return nil
}

func (n *opampNotifier) SendStatus(_ context.Context, message statusNotification) {
	logs := plog.NewLogs()
	log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.Body().SetStr("status")
	attributes := log.Attributes()
	attributes.PutStr("telemetry_type", message.TelemetryType)
	attributes.PutStr("ingest_status", message.IngestStatus)
	attributes.PutInt("start_time", int64(pcommon.NewTimestampFromTime(message.StartTime)))
	attributes.PutInt("end_time", int64(pcommon.NewTimestampFromTime(message.EndTime)))
	log.SetTimestamp(pcommon.NewTimestampFromTime(message.IngestTime))

	if message.FailureMessage != "" {
		attributes.PutStr("failure_message", message.FailureMessage)
	}

	marshaler := plog.ProtoMarshaler{}
	bytes, err := marshaler.MarshalLogs(logs)
	if err != nil {
		return
	}
	for attempt := range maxNotificationAttempts {
		sendingChan, sendingErr := n.handler.SendMessage("TimeBasedIngestStatus", bytes)
		switch {
		case sendingErr == nil:
			return
		case errors.Is(sendingErr, types.ErrCustomMessagePending):
			<-sendingChan
		default:
			// The only other errors returned by the OpAmp extension are unrecoverable, ie ErrCustomCapabilityNotSupported
			// so just log an error and return.
			n.logger.Error("Failed to send notification", zap.Error(sendingErr), zap.Int("attempt", attempt))
			return
		}
	}
	n.logger.Error("Failed to send notification after multiple attempts", zap.Int("max_attempts", maxNotificationAttempts))
}
