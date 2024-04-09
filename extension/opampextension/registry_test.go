package opampextension

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRegistry_Register(t *testing.T) {
	t.Run("Registers successfully", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"

		client := mockCustomCapabilityClient{
			setCustomCapabilites: func(customCapabilities *protobufs.CustomCapabilities) error {
				require.Equal(t,
					&protobufs.CustomCapabilities{
						Capabilities: []string{capabilityString},
					},
					customCapabilities)
				return nil
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		capability, err := registry.Register(capabilityString, func(*protobufs.CustomMessage) {})
		require.NoError(t, err)
		require.NotNil(t, capability)
	})

	t.Run("Setting capabilities fails", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		capabilityErr := errors.New("network error")

		client := mockCustomCapabilityClient{
			setCustomCapabilites: func(customCapabilities *protobufs.CustomCapabilities) error {
				return capabilityErr
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		capability, err := registry.Register(capabilityString, func(*protobufs.CustomMessage) {})
		require.Nil(t, capability)
		require.ErrorIs(t, err, capabilityErr)
		require.Len(t, registry.capabilityToCallbacks, 0, "Setting capability failed, but callback ended up in the map anyways")
	})
}

func TestRegistry_ProcessMessage(t *testing.T) {
	t.Run("Calls registered callback", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		mesageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       mesageBytes,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		callbackCalledChan := make(chan struct{})
		capability, err := registry.Register(capabilityString, func(c *protobufs.CustomMessage) {
			require.Equal(t, customMessage, c)

			close(callbackCalledChan)
		})
		require.NotNil(t, capability)
		require.NoError(t, err)

		registry.ProcessMessage(customMessage)
		select {
		case <-time.After(2 * time.Second):
			t.Fatalf("Timed out waiting for callback to be called")
		case <-callbackCalledChan: // OK
		}

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

		teapotCalledChan := make(chan struct{})
		capabilityTeapot, err := registry.Register(teapotCapabilityString1, func(c *protobufs.CustomMessage) {
			require.Equal(t, customMessageSteep, c)

			close(teapotCalledChan)
		})
		require.NotNil(t, capabilityTeapot)
		require.NoError(t, err)

		coffeeMakerCalledChan := make(chan struct{})
		capabilityCoffeeMaker, err := registry.Register(coffeeMakerCapabilityString2, func(c *protobufs.CustomMessage) {
			require.Equal(t, customMessageBrew, c)

			close(coffeeMakerCalledChan)
		})
		require.NotNil(t, capabilityCoffeeMaker)
		require.NoError(t, err)

		registry.ProcessMessage(customMessageSteep)
		registry.ProcessMessage(customMessageBrew)

		select {
		case <-time.After(2 * time.Second):
			t.Fatalf("Timed out waiting for callback 1 to be called")
		case <-coffeeMakerCalledChan: // OK
		}
		select {
		case <-time.After(2 * time.Second):
			t.Fatalf("Timed out waiting for callback 2 to be called")
		case <-coffeeMakerCalledChan: // OK
		}
	})
}

func TestCustomCapability_SendMesage(t *testing.T) {
	t.Run("Sends message", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "brew"
		mesageBytes := []byte("black")

		client := mockCustomCapabilityClient{
			sendCustomMessage: func(message *protobufs.CustomMessage) (chan struct{}, error) {
				require.Equal(t, &protobufs.CustomMessage{
					Capability: capabilityString,
					Type:       messageType,
					Data:       mesageBytes,
				}, message)
				return nil, nil
			},
		}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		capability, err := registry.Register(capabilityString, func(_ *protobufs.CustomMessage) {})
		require.NoError(t, err)
		require.NotNil(t, capability)

		channel, err := capability.SendMessage(messageType, mesageBytes)
		require.NoError(t, err)
		require.Nil(t, channel, nil)
	})
}

func TestCustomCapability_Unregister(t *testing.T) {
	t.Run("Unregistered capability callback is no longer called", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		mesageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       mesageBytes,
		}

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredCapability, err := registry.Register(capabilityString, func(c *protobufs.CustomMessage) {
			t.Fatalf("Unregistered capability should not be called")
		})
		require.NotNil(t, unregisteredCapability)
		require.NoError(t, err)

		unregisteredCapability.Unregister()

		registry.ProcessMessage(customMessage)
	})

	t.Run("Unregister is successful even if set capabilities fails", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		mesageBytes := []byte("blackTea")
		customMessage := &protobufs.CustomMessage{
			Capability: capabilityString,
			Type:       messageType,
			Data:       mesageBytes,
		}

		client := &mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredCapability, err := registry.Register(capabilityString, func(c *protobufs.CustomMessage) {
			t.Fatalf("Unregistered capability should not be called")
		})
		require.NotNil(t, unregisteredCapability)
		require.NoError(t, err)

		client.setCustomCapabilites = func(customCapabilities *protobufs.CustomCapabilities) error {
			return fmt.Errorf("failed to set capabilities")
		}

		unregisteredCapability.Unregister()

		registry.ProcessMessage(customMessage)
	})

	t.Run("Does not send if unregistered", func(t *testing.T) {
		capabilityString := "io.opentelemetry.teapot"
		messageType := "steep"
		mesageBytes := []byte("blackTea")

		client := mockCustomCapabilityClient{}

		registry := newCustomCapabilityRegistry(zap.NewNop(), client)

		unregisteredCapability, err := registry.Register(capabilityString, func(c *protobufs.CustomMessage) {
			t.Fatalf("Unregistered capability should not be called")
		})
		require.NotNil(t, unregisteredCapability)
		require.NoError(t, err)

		unregisteredCapability.Unregister()

		_, err = unregisteredCapability.SendMessage(messageType, mesageBytes)
		require.ErrorContains(t, err, "capability has already been unregistered")
	})
}

type mockCustomCapabilityClient struct {
	sendCustomMessage    func(message *protobufs.CustomMessage) (chan struct{}, error)
	setCustomCapabilites func(customCapabilities *protobufs.CustomCapabilities) error
}

func (m mockCustomCapabilityClient) SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error {
	if m.setCustomCapabilites != nil {
		return m.setCustomCapabilites(customCapabilities)
	}
	return nil
}

func (m mockCustomCapabilityClient) SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
	if m.sendCustomMessage != nil {
		return m.sendCustomMessage(message)
	}

	return make(chan struct{}), nil
}
