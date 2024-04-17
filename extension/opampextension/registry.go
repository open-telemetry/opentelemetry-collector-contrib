// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"container/list"
	"fmt"
	"slices"
	"sync"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

// customCapabilityClient is a subset of OpAMP client containing only the methods needed by the customCapabilityRegistry
type customCapabilityClient interface {
	SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error
	SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
}

// customCapabilityRegistry implements CustomCapabilityRegistry.
type customCapabilityRegistry struct {
	mux                   *sync.Mutex
	capabilityToListeners map[string]*list.List
	client                customCapabilityClient
	logger                *zap.Logger
	done                  chan struct{}
	listenerWg            *sync.WaitGroup
}

func newCustomCapabilityRegistry(logger *zap.Logger, client customCapabilityClient) *customCapabilityRegistry {
	return &customCapabilityRegistry{
		mux:                   &sync.Mutex{},
		listenerWg:            &sync.WaitGroup{},
		done:                  make(chan struct{}),
		capabilityToListeners: make(map[string]*list.List),
		client:                client,
		logger:                logger,
	}
}

// Register registers a new listener for the custom capability.
func (cr *customCapabilityRegistry) Register(capability string, listener CustomCapabilityListener) (CustomMessageSender, error) {
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

	capabilityList := cr.capabilityToListeners[capability]
	if capabilityList == nil {
		capabilityList = list.New()
		cr.capabilityToListeners[capability] = capabilityList
	}

	listenerElem := capabilityList.PushBack(listener)

	cr.listenerWg.Add(1)
	go cr.waitForListenerDone(capability, listenerElem)

	cc := newCustomCapabilitySender(cr.client, capability)

	return cc, nil
}

// ProcessMessage processes a custom message, asynchronously broadcasting it to all registered listeners for
// the messages capability.
func (cr customCapabilityRegistry) ProcessMessage(cm *protobufs.CustomMessage) {
	cr.mux.Lock()
	defer cr.mux.Unlock()

	listeners, ok := cr.capabilityToListeners[cm.Capability]
	if !ok {
		return
	}

	for node := listeners.Front(); node != nil; node = node.Next() {
		listener, ok := node.Value.(CustomCapabilityListener)
		if !ok {
			continue
		}

		// Let the listener process asynchronously in a separate goroutine so it can't block
		// the opamp extension
		go listener.ReceiveMessage(cm)
	}
}

// waitForListenerDone waits for the CustomCapabilityListener inside the listenerElem to emit a Done signal down it's channel,
// at which point the listener will be unregistered.
func (cr *customCapabilityRegistry) waitForListenerDone(capability string, listenerElem *list.Element) {
	defer cr.listenerWg.Done()

	listener := listenerElem.Value.(CustomCapabilityListener)

	select {
	case <-listener.Done():
	case <-cr.done:
		// This means the whole extension is shutting down, so we don't need to modify the custom capabilities
		// (since we are disconnected at this point), and we just need to clean up this goroutine.
		return
	}

	cr.removeCustomCapabilityListener(capability, listenerElem)
}

// removeCustomCapabilityListener removes the custom capability with the given listener list element,
// then recalculates and sets the list of custom capabilities on the OpAMP client.
func (cr *customCapabilityRegistry) removeCustomCapabilityListener(capability string, listenerElement *list.Element) {
	cr.mux.Lock()
	defer cr.mux.Unlock()

	listenerList := cr.capabilityToListeners[capability]
	listenerList.Remove(listenerElement)

	if listenerList.Front() == nil {
		// Since there are no more listeners for this capability,
		// this capability is no longer supported
		delete(cr.capabilityToListeners, capability)
	}

	capabilities := cr.capabilities()
	err := cr.client.SetCustomCapabilities(&protobufs.CustomCapabilities{
		Capabilities: capabilities,
	})
	if err != nil {
		// It's OK if we couldn't actually remove the capability, it just means we won't
		// notify the server properly, and the server may send us messages that we have no associated listeners for.
		cr.logger.Error("Failed to set new capabilities", zap.Error(err))
	}
}

// capabilities gives the current set of custom capabilities with at least one
// listener registered.
func (cr *customCapabilityRegistry) capabilities() []string {
	return maps.Keys(cr.capabilityToListeners)
}

// Stop unregisters all registered capabilities, freeing all goroutines occupied waiting for listeners to emit their Done signal.
func (cr *customCapabilityRegistry) Stop() {
	close(cr.done)
	cr.listenerWg.Wait()
}

// customCapabilitySender implements
type customCapabilitySender struct {
	capability  string
	opampClient customCapabilityClient
}

func newCustomCapabilitySender(
	opampClient customCapabilityClient,
	capability string,
) *customCapabilitySender {
	return &customCapabilitySender{
		capability:  capability,
		opampClient: opampClient,
	}
}

func (c *customCapabilitySender) SendMessage(messageType string, message []byte) (messageSendingChannel chan struct{}, err error) {
	cm := &protobufs.CustomMessage{
		Capability: c.capability,
		Type:       messageType,
		Data:       message,
	}

	return c.opampClient.SendCustomMessage(cm)
}
