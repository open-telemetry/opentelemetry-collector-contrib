// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"sync"

	"go.opentelemetry.io/collector/component"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

type Registry interface {
	// Load the Sender for the ID.
	Load(id component.ID) Sender
	// LoadOrNop gets the Sender for the ID. If it doesn't exist, returns
	// the NopSender.
	LoadOrNop(id component.ID) Sender
	// LoadOrStore the Sender for the ID.
	LoadOrStore(id component.ID, sender Sender) (Sender, bool)
	// Register configures and registers a new Sender for the ID. If one
	// already exists for the ID, then returns that one instead.
	Register(id component.ID, cfg Config, client awsxray.XRayClient, opts ...Option) Sender
}

var globalRegistry = NewRegistry()

// GlobalRegistry returns the global Registry.
func GlobalRegistry() Registry {
	return globalRegistry
}

// registry maintains a map of all registered senders.
type registry struct {
	senders sync.Map
}

// NewRegistry returns a new empty Registry.
func NewRegistry() Registry {
	return &registry{}
}

func (r *registry) Load(id component.ID) Sender {
	sender, ok := r.senders.Load(id)
	if ok {
		return sender.(Sender)
	}
	return nil
}

func (r *registry) LoadOrNop(id component.ID) Sender {
	sender := r.Load(id)
	if sender == nil {
		sender = NewNopSender()
	}
	return sender
}

func (r *registry) LoadOrStore(id component.ID, sender Sender) (Sender, bool) {
	actual, loaded := r.senders.LoadOrStore(id, sender)
	return actual.(Sender), loaded
}

func (r *registry) Register(
	id component.ID,
	cfg Config,
	client awsxray.XRayClient,
	opts ...Option,
) Sender {
	if sender, ok := r.senders.Load(id); ok {
		return sender.(Sender)
	}
	sender := NewSender(client, opts...)
	r.senders.Store(id, sender)
	for _, contributor := range cfg.Contributors {
		r.LoadOrStore(contributor, sender)
	}
	return sender
}
