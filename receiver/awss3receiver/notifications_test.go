// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

type mockCustomCapabilityRegistry struct {
	component.Component

	shouldFailRegister  bool
	shouldReturnPending bool

	pendingChannel   chan struct{}
	unregisterCalled bool
	sentMessages     []customMessage
}

type customMessage struct {
	messageType string
	message     []byte
}

type hostWithExtensions struct {
	extension *mockCustomCapabilityRegistry
}

func (h hostWithExtensions) Start(context.Context, component.Host) error {
	panic("unsupported")
}

func (h hostWithExtensions) Shutdown(context.Context) error {
	panic("unsupported")
}

func (h hostWithExtensions) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	panic("unsupported")
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("foo"): h.extension,
	}
}

func (h hostWithExtensions) GetExporters() map[component.DataType]map[component.ID]component.Component {
	panic("unsupported")
}

func (m *mockCustomCapabilityRegistry) Register(_ string, _ ...opampcustommessages.CustomCapabilityRegisterOption) (handler opampcustommessages.CustomCapabilityHandler, err error) {
	if m.shouldFailRegister {
		return nil, fmt.Errorf("register failed")
	}
	return m, nil
}

func (m *mockCustomCapabilityRegistry) Message() <-chan *protobufs.CustomMessage {
	panic("unsupported")
}

func (m *mockCustomCapabilityRegistry) SendMessage(messageType string, message []byte) (messageSendingChannel chan struct{}, err error) {
	if m.unregisterCalled {
		return nil, fmt.Errorf("unregister called")
	}
	m.sentMessages = append(m.sentMessages, customMessage{messageType: messageType, message: message})
	if m.shouldReturnPending {
		return m.pendingChannel, types.ErrCustomMessagePending
	}
	return nil, nil
}

func (m *mockCustomCapabilityRegistry) Unregister() {
	m.unregisterCalled = true
}

func Test_opampNotifier_Start(t *testing.T) {
	id := component.MustNewID("foo")

	tests := []struct {
		name    string
		host    component.Host
		wantErr bool
	}{
		{
			name: "success",
			host: hostWithExtensions{
				extension: &mockCustomCapabilityRegistry{},
			},
			wantErr: false,
		},
		{
			name:    "extension not found",
			host:    componenttest.NewNopHost(),
			wantErr: true,
		},
		{
			name: "register failed",
			host: hostWithExtensions{
				extension: &mockCustomCapabilityRegistry{
					shouldFailRegister: true,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := &opampNotifier{opampExtensionID: id}
			err := notifier.Start(context.Background(), tt.host)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_opampNotifier_Shutdown(t *testing.T) {
	registry := mockCustomCapabilityRegistry{}
	notifier := &opampNotifier{handler: &registry}
	err := notifier.Shutdown(context.Background())
	require.NoError(t, err)
	require.True(t, registry.unregisterCalled)
}

func Test_opampNotifier_SendStatus(t *testing.T) {
	registry := mockCustomCapabilityRegistry{}
	notifier := &opampNotifier{handler: &registry}
	toSend := StatusNotification{
		TelemetryType: "telemetry",
		IngestStatus:  IngestStatusIngesting,
		IngestTime:    time.Time{},
	}
	notifier.SendStatus(context.Background(), toSend)
	require.Len(t, registry.sentMessages, 1)
	require.Equal(t, "TimeBasedIngestStatus", registry.sentMessages[0].messageType)
	got := StatusNotification{}
	err := json.Unmarshal(registry.sentMessages[0].message, &got)
	require.NoError(t, err)
	require.Equal(t, toSend, got)
}

func Test_opampNotifier_SendStatus_MessagePending(t *testing.T) {
	registry := mockCustomCapabilityRegistry{
		shouldReturnPending: true,
		pendingChannel:      make(chan struct{}),
	}
	notifier := &opampNotifier{handler: &registry}
	toSend := StatusNotification{
		TelemetryType: "telemetry",
		IngestStatus:  IngestStatusIngesting,
		IngestTime:    time.Time{},
	}

	var completionTime time.Time
	now := time.Now()
	go func() {
		time.Sleep(10 * time.Millisecond)
		registry.pendingChannel <- struct{}{}
	}()
	notifier.SendStatus(context.Background(), toSend)
	completionTime = time.Now()
	require.True(t, completionTime.After(now))
	require.Len(t, registry.sentMessages, 1)
	require.Equal(t, "TimeBasedIngestStatus", registry.sentMessages[0].messageType)
	got := StatusNotification{}
	err := json.Unmarshal(registry.sentMessages[0].message, &got)
	require.NoError(t, err)
	require.Equal(t, toSend, got)
}
