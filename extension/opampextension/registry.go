package opampextension

import (
	"container/list"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

// customCapabilityClient is a subset of OpAMP client containing only the
type customCapabilityClient interface {
	SetCustomCapabilities(customCapabilities *protobufs.CustomCapabilities) error
	SendCustomMessage(message *protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error)
}

type customCapabilityRegistry struct {
	mux                   *sync.Mutex
	capabilityToCallbacks map[string]*list.List
	client                customCapabilityClient
	logger                *zap.Logger
}

func newCustomCapabilityRegistry(logger *zap.Logger, client customCapabilityClient) *customCapabilityRegistry {
	return &customCapabilityRegistry{
		mux:                   &sync.Mutex{},
		capabilityToCallbacks: make(map[string]*list.List),
		client:                client,
		logger:                logger,
	}
}

// Register registers a new custom capability for the
func (cr *customCapabilityRegistry) Register(capability string, callback CustomMessageCallback) (CustomCapability, error) {
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

	capabilityList := cr.capabilityToCallbacks[capability]
	if capabilityList == nil {
		capabilityList = list.New()
		cr.capabilityToCallbacks[capability] = capabilityList
	}

	callbackElem := capabilityList.PushBack(callback)

	cc := newCustomCapability(cr, cr.client, capability)

	// note: We'll have to set the self element in order for the custom capability to remove itself.
	cc.selfElement = callbackElem

	return cc, nil
}

// ProcessMessage processes a custom message, asynchronously broadcasting it to all registered callbacks for
// the messages capability.
func (cr customCapabilityRegistry) ProcessMessage(cm *protobufs.CustomMessage) {
	cr.mux.Lock()
	defer cr.mux.Unlock()

	callbacks, ok := cr.capabilityToCallbacks[cm.Capability]
	if !ok {
		return
	}

	for node := callbacks.Front(); node != nil; node = node.Next() {
		cb, ok := node.Value.(CustomMessageCallback)
		if !ok {
			continue
		}

		// Let the callback process asynchronously in a separate goroutine so it can't block
		// the opamp extension
		go cb(cm)
	}
}

// RemoveCustomCapability removes the custom capability with the given callback list element,
// then recalculates and sets the list of custom capabilities on the OpAMP client.
func (cr *customCapabilityRegistry) RemoveCustomCapability(capability string, callbackElement *list.Element) {
	cr.mux.Lock()
	defer cr.mux.Unlock()

	callbackList := cr.capabilityToCallbacks[capability]
	callbackList.Remove(callbackElement)

	if callbackList.Front() == nil {
		// Since there are no more callbacks for this capability,
		// this capability is no longer supported
		delete(cr.capabilityToCallbacks, capability)
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

// capabilities gives the current set of custom capabilities with at least one
// callback registered.
func (cr *customCapabilityRegistry) capabilities() []string {
	return maps.Keys(cr.capabilityToCallbacks)
}

type customCapabilityHandler struct {
	// unregisteredMux protects unregistered, and makes sure that a message cannot be sent
	// on an unregistered capability.
	unregisteredMux *sync.Mutex

	capability   string
	selfElement  *list.Element
	opampClient  customCapabilityClient
	registry     *customCapabilityRegistry
	unregistered bool
}

func newCustomCapability(
	registry *customCapabilityRegistry,
	opampClient customCapabilityClient,
	capability string,
) *customCapabilityHandler {
	return &customCapabilityHandler{
		unregisteredMux: &sync.Mutex{},

		capability:  capability,
		opampClient: opampClient,
		registry:    registry,
	}
}

// SendMessage synchronously sends the message
func (c *customCapabilityHandler) SendMessage(messageType string, message []byte) (messageSendingChannel chan struct{}, err error) {
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

func (c *customCapabilityHandler) Unregister() {
	c.unregisteredMux.Lock()
	defer c.unregisteredMux.Unlock()

	c.unregistered = true
	c.registry.RemoveCustomCapability(c.capability, c.selfElement)
}
