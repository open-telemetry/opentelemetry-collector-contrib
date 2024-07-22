// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/remotetap"
)

// CallbackManager registers components and callbacks that can be used to send data to observers.
type CallbackManager struct {
	mu        sync.RWMutex
	callbacks map[remotetap.ComponentID]map[CallbackID]func(string)
}

func NewCallbackManager() *CallbackManager {
	return &CallbackManager{
		callbacks: make(map[remotetap.ComponentID]map[CallbackID]func(string)),
	}
}

// Add adds a new callback for a registered component.
func (cm *CallbackManager) Add(componentID remotetap.ComponentID, callbackID CallbackID, callback func(string)) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.callbacks[componentID]; !ok {
		return fmt.Errorf("componentID %q is not registered to the remoteTap extension", componentID)
	}

	cm.callbacks[componentID][callbackID] = callback
	return nil
}

// Delete deletes a new callback for a registered component.
func (cm *CallbackManager) Delete(componentID remotetap.ComponentID, callbackID CallbackID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.callbacks[componentID], callbackID)
}

// Register registers a component to the extension.
func (cm *CallbackManager) Register(componentID remotetap.ComponentID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.callbacks[componentID] = make(map[CallbackID]func(string))
}

// IsActive returns true is the given component has a least one active observer.
func (cm *CallbackManager) IsActive(componentID remotetap.ComponentID) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	callbacks, exist := cm.callbacks[componentID]
	return exist && len(callbacks) > 0
}

// Broadcast sends data from a registered component to its observers.
func (cm *CallbackManager) Broadcast(componentID remotetap.ComponentID, data string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, callback := range cm.callbacks[componentID] {
		callback(data)
	}
}

// GetRegisteredComponents returns all registered components.
func (cm *CallbackManager) GetRegisteredComponents() []remotetap.ComponentID {
	components := make([]remotetap.ComponentID, 0, len(cm.callbacks))
	for componentID := range cm.callbacks {
		components = append(components, componentID)
	}
	return components
}
