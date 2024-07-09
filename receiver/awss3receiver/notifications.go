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
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

const (
	IngestStatusCompleted = "completed"
	IngestStatusFailed    = "failed"
	IngestStatusIngesting = "ingesting"
	CustomCapability      = "org.opentelemetry.collector.receiver.awss3"
)

type StatusNotification struct {
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
	SendStatus(ctx context.Context, message StatusNotification)
}

type opampNotifier struct {
	opampExtensionID component.ID
	handler          opampcustommessages.CustomCapabilityHandler
}

func newNotifier(config *Config) statusNotifier {
	if config.Notifications.OpAMP != nil {
		return &opampNotifier{opampExtensionID: *config.Notifications.OpAMP}
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
	n.handler = handler
	return nil
}

func (n *opampNotifier) Shutdown(_ context.Context) error {
	n.handler.Unregister()
	return nil
}

func (n *opampNotifier) SendStatus(_ context.Context, message StatusNotification) {
	logs := plog.NewLogs()
	log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.Body().SetStr("status")
	attributes := log.Attributes()
	attributes.PutStr("telemetry_type", message.TelemetryType)
	attributes.PutStr("ingest_status", message.IngestStatus)
	attributes.PutStr("start_time", message.StartTime.Format(time.RFC3339))
	attributes.PutStr("end_time", message.EndTime.Format(time.RFC3339))
	attributes.PutStr("ingest_time", message.IngestTime.Format(time.RFC3339))
	if message.FailureMessage != "" {
		attributes.PutStr("failure_message", message.FailureMessage)
	}

	marshaler := plog.ProtoMarshaler{}
	bytes, err := marshaler.MarshalLogs(logs)
	if err != nil {
		return
	}
	sendingChan, err := n.handler.SendMessage("TimeBasedIngestStatus", bytes)
	switch {
	case err == nil:
		break
	case errors.Is(err, types.ErrCustomMessagePending):
		<-sendingChan
	default:
		return
	}
}
