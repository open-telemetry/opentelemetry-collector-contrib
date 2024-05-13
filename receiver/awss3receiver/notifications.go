// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opamp-go/client/types"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

const (
	IngestStatusCompleted = "completed"
	IngestStatusFailed    = "failed"
	IngestStatusIngesting = "ingesting"
	CustomCapability      = "org.opentelemetry.collector.receiver.awss3"
)

type StatusNotification struct {
	TelemetryType  string    `json:"telemetry_type"`
	IngestStatus   string    `json:"ingest_status"`
	IngestTime     time.Time `json:"ingest_time"`
	FailureMessage string    `json:"failure_message,omitempty"`
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
	bytes, err := json.Marshal(message)
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
