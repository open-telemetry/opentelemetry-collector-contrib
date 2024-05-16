// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"container/list"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages"
)

// customCapabilityClient is a subset of OpAMP client containing only the methods needed for the customCapabilityRegistry.
type customCapabilityClient interface {
	SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error
	SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
}

type customCapabilityRegistry struct {
	mux                     *sync.Mutex
	capabilityToMsgChannels map[string]*list.List
	client                  customCapabilityClient
	logger                  *zap.Logger
}

var _ opampcustommessages.CustomCapabilityRegistry = (*customCapabilityRegistry)(nil)

func newCustomCapabilityRegistry(logger *zap.Logger, client customCapabilityClient) *customCapabilityRegistry {
	return &customCapabilityRegistry{
		mux:                     &sync.Mutex{},
		capabilityToMsgChannels: make(map[string]*list.List),
		client:                  client,
		logger:                  logger,
	}
}

// Register implements CustomCapabilityRegistry.Register
func (cr *customCapabilityRegistry) Register(capability string, opts ...opampcustommessages.CustomCapabilityRegisterOption) (opampcustommessages.CustomCapabilityHandler, error) {
	optsStruct := opampcustommessages.DefaultCustomCapabilityRegisterOptions()
	for _, opt := range opts {
		opt(optsStruct)
	}

	cr.mux.Lock()
	defer cr.mux.Unlock()

	capabilities := cr.capabilities()
	if !slices.Contains(capabilities, capability) {
		capabilities = append(capabilities, capability)
	}

	err := cr.client.SetCustomCapabilities(&protobufs.CustomCapabilities{
		Capabilities: capabilities,
	})
	if err != nil {
		return nil, fmt.Errorf("set custom capabilities: %w", err)
	}

	capabilityList := cr.capabilityToMsgChannels[capability]
	if capabilityList == nil {
		capabilityList = list.New()
		cr.capabilityToMsgChannels[capability] = capabilityList
	}

	msgChan := make(chan *protobufs.CustomMessage, optsStruct.MaxQueuedMessages)
	callbackElem := capabilityList.PushBack(msgChan)

	unregisterFunc := cr.removeCapabilityFunc(capability, callbackElem)
	sender := newCustomMessageHandler(cr, cr.client, capability, msgChan, unregisterFunc)

	return sender, nil
}

// ProcessMessage processes a custom message, asynchronously broadcasting it to all registered capability handlers for
// the messages capability.
func (cr customCapabilityRegistry) ProcessMessage(cm *protobufs.CustomMessage) {
	cr.mux.Lock()
	defer cr.mux.Unlock()

	msgChannels, ok := cr.capabilityToMsgChannels[cm.Capability]
	if !ok {
		return
	}

	for node := msgChannels.Front(); node != nil; node = node.Next() {
		msgChan, ok := node.Value.(chan *protobufs.CustomMessage)
		if !ok {
			continue
		}

		// If the channel is full, we will skip sending the message to the receiver.
		// We do this because we don't want a misbehaving component to be able to
		// block the opamp extension, or block other components from receiving messages.
		select {
		case msgChan <- cm:
		default:
		}
	}
}

// removeCapabilityFunc returns a func that removes the custom capability with the given msg channel list element and sender,
// then recalculates and sets the list of custom capabilities on the OpAMP client.
func (cr *customCapabilityRegistry) removeCapabilityFunc(capability string, callbackElement *list.Element) func() {
	return func() {
		cr.mux.Lock()
		defer cr.mux.Unlock()

		msgChanList := cr.capabilityToMsgChannels[capability]
		msgChanList.Remove(callbackElement)

		if msgChanList.Front() == nil {
			// Since there are no more callbacks for this capability,
			// this capability is no longer supported
			delete(cr.capabilityToMsgChannels, capability)
		}

		capabilities := cr.capabilities()
		err := cr.client.SetCustomCapabilities(&protobufs.CustomCapabilities{
			Capabilities: capabilities,
		})
		if err != nil {
			// It's OK if we couldn't actually remove the capability, it just means we won't
			// notify the server properly, and the server may send us messages that we have no associated callbacks for.
			cr.logger.Error("Failed to set new capabilities", zap.Error(err))
		}
	}
}

// capabilities gives the current set of custom capabilities with at least one
// callback registered.
func (cr *customCapabilityRegistry) capabilities() []string {
	return maps.Keys(cr.capabilityToMsgChannels)
}

type customMessageHandler struct {
	// unregisteredMux protects unregistered, and makes sure that a message cannot be sent
	// on an unregistered capability.
	unregisteredMux *sync.Mutex

	capability               string
	opampClient              customCapabilityClient
	registry                 *customCapabilityRegistry
	sendChan                 <-chan *protobufs.CustomMessage
	unregisterCapabilityFunc func()

	unregistered bool
}

var _ opampcustommessages.CustomCapabilityHandler = (*customMessageHandler)(nil)

func newCustomMessageHandler(
	registry *customCapabilityRegistry,
	opampClient customCapabilityClient,
	capability string,
	sendChan <-chan *protobufs.CustomMessage,
	unregisterCapabilityFunc func(),
) *customMessageHandler {
	return &customMessageHandler{
		unregisteredMux: &sync.Mutex{},

		capability:               capability,
		opampClient:              opampClient,
		registry:                 registry,
		sendChan:                 sendChan,
		unregisterCapabilityFunc: unregisterCapabilityFunc,
	}
}

// Message implements CustomCapabilityHandler.Message
func (c *customMessageHandler) Message() <-chan *protobufs.CustomMessage {
	return c.sendChan
}

// SendMessage implements CustomCapabilityHandler.SendMessage
func (c *customMessageHandler) SendMessage(messageType string, message []byte) (messageSendingChannel chan struct{}, err error) {
	c.unregisteredMux.Lock()
	defer c.unregisteredMux.Unlock()

	if c.unregistered {
		return nil, errors.New("capability has already been unregistered")
	}

	cm := &protobufs.CustomMessage{
		Capability: c.capability,
		Type:       messageType,
		Data:       message,
	}

	return c.opampClient.SendCustomMessage(cm)
}

// Unregister implements CustomCapabilityHandler.Unregister
func (c *customMessageHandler) Unregister() {
	c.unregisteredMux.Lock()
	defer c.unregisteredMux.Unlock()

	c.unregistered = true

	c.unregisterCapabilityFunc()
}
