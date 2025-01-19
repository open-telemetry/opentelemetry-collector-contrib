// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package opampcustommessages contains interfaces and shared code for sending and receiving custom messages via OpAMP.
package opampcustommessages // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"

import "github.com/open-telemetry/opamp-go/protobufs"

// CustomCapabilityRegisterOptions represents extra options that can be use in CustomCapabilityRegistry.Register
type CustomCapabilityRegisterOptions struct {
	MaxQueuedMessages int
}

// DefaultCustomCapabilityRegisterOptions returns the default options for CustomCapabilityRegisterOptions
func DefaultCustomCapabilityRegisterOptions() *CustomCapabilityRegisterOptions {
	return &CustomCapabilityRegisterOptions{
		MaxQueuedMessages: 10,
	}
}

// CustomCapabilityRegisterOption represent a single option for CustomCapabilityRegistry.Register
type CustomCapabilityRegisterOption func(*CustomCapabilityRegisterOptions)

// WithMaxQueuedMessages overrides the maximum number of queued messages. If a message is received while
// MaxQueuedMessages messages are already queued to be processed, the message is dropped.
func WithMaxQueuedMessages(maxQueuedMessages int) CustomCapabilityRegisterOption {
	return func(c *CustomCapabilityRegisterOptions) {
		c.MaxQueuedMessages = maxQueuedMessages
	}
}

// CustomCapabilityRegistry allows for registering a custom capability that can receive custom messages.
type CustomCapabilityRegistry interface {
	// Register registers a new custom capability.
	// It returns a CustomMessageHandler, which can be used to send and receive
	// messages to/from the OpAMP server.
	Register(capability string, opts ...CustomCapabilityRegisterOption) (handler CustomCapabilityHandler, err error)
}

// CustomCapabilityHandler represents a handler for a custom capability.
// This can be used to send and receive custom messages to/from an OpAMP server.
// It can also be used to unregister the custom capability when it is no longer supported.
type CustomCapabilityHandler interface {
	// Message returns a channel that can be used to receive custom messages sent from the OpAMP server.
	Message() <-chan *protobufs.CustomMessage

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

	// Unregister unregisters the custom capability. After this method returns, SendMessage will always return an error,
	// and Message will no longer receive further custom messages.
	Unregister()
}
