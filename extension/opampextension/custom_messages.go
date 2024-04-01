package opampextension

import (
	"github.com/open-telemetry/opamp-go/protobufs"
)

// CustomMessageCallback is a callback that handles a custom message from opamp.
type CustomMessageCallback func(*protobufs.CustomMessage)

// CustomCapabilityRegistry allows for registering a custom capability that can receive custom messages.
type CustomCapabilityRegistry interface {
	// Register registers a new custom capability. Any messages for the capability
	// will be received by the given callback asynchronously.
	// It returns a handle to a CustomCapability, which can be used to unregister, or send
	// a message to the OpAMP server.
	Register(capability string, callback CustomMessageCallback) (CustomCapability, error)
}

// CustomCapability represents a handle to a custom capability.
// This can be used to send a custom message to an OpAMP server, or to unregister
// the capability from the registry.
type CustomCapability interface {
	// SendMessage sends a custom message to the OpAMP server.
	//
	// Only one message can be sent at a time. If SendCustomMessage has been already called
	// for any capability, and the message is still pending (in progress)
	// then subsequent calls to SendCustomMessage will return github.com/open-telemetry/opamp-go/types.ErrCustomMessagePending
	// and a channel that will be closed when the pending message is sent.
	// To ensure that the previous send is complete and it is safe to send another CustomMessage,
	// the caller should wait for the returned channel to be closed before attempting to send another custom message.
	//
	// If no error is returned, the channel returned will be closed after the specified
	// message is sent.
	SendMessage(messageType string, message []byte) (messageSendingChannel chan struct{}, err error)

	// Unregister unregisters the custom capability.
	Unregister()
}
