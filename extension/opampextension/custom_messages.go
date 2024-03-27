package opampextension

import "github.com/open-telemetry/opamp-go/protobufs"

// CustomMessageCallback is a callback that handles a custom message from opamp.
type CustomMessageCallback func(*protobufs.CustomMessage)

// CustomCapabilityRegistry allows for registering a custom capability that can receive custom messages.
type CustomCapabilityRegistry interface {
	// Register registers a new custom capability. Any messages for the capability
	// will be received by the given callback asynchronously.
	// It returns a handle to a CustomCapability, which can be used to unregister, or send
	// a message to the OpAMP server.
	Register(capability string, callback CustomMessageCallback) CustomCapability
}

// CustomCapability represents a handle to a custom capability.
// This can be used to send a custom message to an OpAMP server, or to unregister
// the capability from the registry.
type CustomCapability interface {
	// SendMessage sends a custom message to the OpAMP server.
	SendMessage(messageType string, message []byte) error
	// Unregister unregisters the custom capability.
	Unregister()
}
