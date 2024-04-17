// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"github.com/open-telemetry/opamp-go/protobufs"
)

// CustomMessageCallback is a callback that handles a custom message from OpAMP.
type CustomMessageCallback func(*protobufs.CustomMessage)

// CustomCapabilityRegistry allows for registering a custom capability that can receive custom messages.
type CustomCapabilityRegistry interface {
	// Register registers a new custom capability. Any messages for the capability
	// will be received by the given callback asynchronously.
	// It returns a a CustomCapabilitySender, which can be used to send a message to the OpAMP server using the capability.
	Register(capability string, listener CustomCapabilityListener) (CustomMessageSender, error)
}

// CustomCapabilityListener is an interface that receives custom messages.
type CustomCapabilityListener interface {
	// ReceiveMessage is called whenever a message for the capability is received from the OpAMP server.
	ReceiveMessage(msg *protobufs.CustomMessage)
	// Done returns a channel signaling that the listener is done listening. Some time after a signal is emitted on the channel,
	// messages will no longer be received by ReceiveMessage for this listener.
	Done() <-chan struct{}
}

// CustomMessageSender can be used to send a custom message to an OpAMP server.
// The capability the message is sent for is the capability that was used in CustomCapabilityRegistry.Register.
type CustomMessageSender interface {
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
}
