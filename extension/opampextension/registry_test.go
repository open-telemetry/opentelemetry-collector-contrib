// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"errors"
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

func TestRegistry_Register(t *testing.T) {
	t.Run("Registers successfully", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"

		client := mockCustomCapabilityClient{
			setCustomCapabilities: func(customCapabilities *protobufs.CustomCapabilities) error {
				require.Equal(t,
					&protobufs.CustomCapabilities{
						Capabilities: []string{capabilityString},
					},
					customCapabilities)
				return nil
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		sender, err := registry.Register(capabilityString)
		require.NoError(t, err)
		require.NotNil(t, sender)
	})

	t.Run("Setting capabilities fails", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		capabilityErr := errors.New("network error")

		client := mockCustomCapabilityClient{
			setCustomCapabilities: func(_ *protobufs.CustomCapabilities) error {
				return capabilityErr
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		sender, err := registry.Register(capabilityString)
		require.Nil(t, sender)
		require.ErrorIs(t, err, capabilityErr)
		require.Empty(t, registry.capabilityToMsgChannels, "Setting capability failed, but callback ended up in the map anyways")
	})
}

func TestRegistry_ProcessMessage(t *testing.T) {
	t.Run("Calls registered callback", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		messageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       messageBytes,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		sender, err := registry.Register(capabilityString)
		require.NotNil(t, sender)
		require.NoError(t, err)

		registry.ProcessMessage(customMessage)

		require.Equal(t, customMessage, <-sender.Message())
	})

	t.Run("Skips blocked message channels", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		messageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       messageBytes,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		sender, err := registry.Register(capabilityString, opampcustommessages.WithMaxQueuedMessages(0))
		require.NotNil(t, sender)
		require.NoError(t, err)

		// If we did not skip sending on blocked channels, we'd expect this to never return.
		registry.ProcessMessage(customMessage)

		require.Empty(t, sender.Message())
	})

	t.Run("Callback is called only for its own capability", func(t *testing.T) {
		teapotCapabilityString1 := "io.opentelemetry.teapot"
		coffeeMakerCapabilityString2 := "io.opentelemetry.coffeeMaker"

		messageType1 := "steep"
		messageBytes1 := []byte("blackTea")

		messageType2 := "brew"
		messageBytes2 := []byte("blackCoffee")

		customMessageSteep := &protobufs.CustomMessage{
			Capability: teapotCapabilityString1,
			Type:       messageType1,
			Data:       messageBytes1,
		}

		customMessageBrew := &protobufs.CustomMessage{
			Capability: coffeeMakerCapabilityString2,
			Type:       messageType2,
			Data:       messageBytes2,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		teapotSender, err := registry.Register(teapotCapabilityString1)
		require.NotNil(t, teapotSender)
		require.NoError(t, err)

		coffeeMakerSender, err := registry.Register(coffeeMakerCapabilityString2)
		require.NotNil(t, coffeeMakerSender)
		require.NoError(t, err)

		registry.ProcessMessage(customMessageSteep)
		registry.ProcessMessage(customMessageBrew)

		require.Equal(t, customMessageSteep, <-teapotSender.Message())
		require.Empty(t, teapotSender.Message())
		require.Equal(t, customMessageBrew, <-coffeeMakerSender.Message())
		require.Empty(t, coffeeMakerSender.Message())
	})
}

func TestCustomCapability_SendMessage(t *testing.T) {
	t.Run("Sends message", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "brew"
		messageBytes := []byte("black")

		client := mockCustomCapabilityClient{
			sendCustomMessage: func(message *protobufs.CustomMessage) (chan struct{}, error) {
				require.Equal(t, &protobufs.CustomMessage{
					Capability: capabilityString,
					Type:       messageType,
					Data:       messageBytes,
				}, message)
				return nil, nil
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		sender, err := registry.Register(capabilityString)
		require.NoError(t, err)
		require.NotNil(t, sender)

		channel, err := sender.SendMessage(messageType, messageBytes)
		require.NoError(t, err)
		require.Nil(t, channel)
	})
}

func TestCustomCapability_Unregister(t *testing.T) {
	t.Run("Unregistered capability callback is no longer called", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		messageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       messageBytes,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredSender, err := registry.Register(capabilityString)
		require.NotNil(t, unregisteredSender)
		require.NoError(t, err)

		unregisteredSender.Unregister()

		registry.ProcessMessage(customMessage)

		select {
		case <-unregisteredSender.Message():
			t.Fatalf("Unregistered capability should not be called")
		default: // OK
		}
	})

	t.Run("Unregister is successful even if set capabilities fails", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		messageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       messageBytes,
		}

		client := &mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredSender, err := registry.Register(capabilityString)
		require.NotNil(t, unregisteredSender)
		require.NoError(t, err)

		client.setCustomCapabilities = func(_ *protobufs.CustomCapabilities) error {
			return errors.New("failed to set capabilities")
		}

		unregisteredSender.Unregister()

		registry.ProcessMessage(customMessage)

		select {
		case <-unregisteredSender.Message():
			t.Fatalf("Unregistered capability should not be called")
		default: // OK
		}
	})

	t.Run("Does not send if unregistered", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		messageBytes := []byte("blackTea")

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredSender, err := registry.Register(capabilityString)
		require.NotNil(t, unregisteredSender)
		require.NoError(t, err)

		unregisteredSender.Unregister()

		_, err = unregisteredSender.SendMessage(messageType, messageBytes)
		require.ErrorContains(t, err, "capability has already been unregistered")

		select {
		case <-unregisteredSender.Message():
			t.Fatalf("Unregistered capability should not be called")
		default: // OK
		}
	})
}

type mockCustomCapabilityClient struct {
	sendCustomMessage     func(message *protobufs.CustomMessage) (chan struct{}, error)
	setCustomCapabilities func(customCapabilities *protobufs.CustomCapabilities) error
}

func (m mockCustomCapabilityClient) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	if m.setCustomCapabilities != nil {
		return m.setCustomCapabilities(customCapabilities)
	}
	return nil
}

func (m mockCustomCapabilityClient) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	if m.sendCustomMessage != nil {
		return m.sendCustomMessage(message)
	}

	return make(chan struct{}), nil
}
