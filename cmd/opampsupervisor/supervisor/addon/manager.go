// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// Manager holds the addon types the Supervisor supports and the addons
// installed on this host.
//
// All methods are serialized: package syncing, remote config messages and
// startup all reconcile through the Manager, possibly from different
// goroutines, and an addon's methods are never called concurrently.
type Manager struct {
	logger    *zap.Logger
	factories map[Type]Factory

	mu        sync.Mutex
	instances map[string]*instance
}

// instance is an installed addon and the configuration last applied to it.
type instance struct {
	id    ID
	dir   string
	addon Addon
	// configured is false until Configure has been called once, so that the
	// first reconciliation always configures, even with a nil body.
	configured bool
	lastConfig []byte
}

// NewManager returns a Manager supporting the given addon types.
func NewManager(logger *zap.Logger, factories ...Factory) (*Manager, error) {
	m := &Manager{
		logger:    logger,
		factories: make(map[Type]Factory, len(factories)),
		instances: map[string]*instance{},
	}
	for _, f := range factories {
		typ := f.Type()
		if err := typ.Validate(); err != nil {
			return nil, err
		}
		if _, ok := m.factories[typ]; ok {
			return nil, fmt.Errorf("addon type %q is registered more than once", typ)
		}
		m.factories[typ] = f
	}
	return m, nil
}

// Types returns the supported addon types, sorted.
func (m *Manager) Types() []Type {
	types := make([]Type, 0, len(m.factories))
	for typ := range m.factories {
		types = append(types, typ)
	}
	slices.Sort(types)
	return types
}

// Supports reports whether packageName names an addon of a supported type. It
// is checked before anything is downloaded, so an offer of an unsupported type
// fails without fetching it.
func (m *Manager) Supports(packageName string) error {
	_, _, err := m.factoryFor(packageName)
	return err
}

func (m *Manager) factoryFor(packageName string) (ID, Factory, error) {
	id, err := ParseID(packageName)
	if err != nil {
		return ID{}, nil, err
	}
	f, ok := m.factories[id.Type]
	if !ok {
		return ID{}, nil, fmt.Errorf("unsupported addon type %q in package %q; supported types are %v",
			id.Type, packageName, m.Types())
	}
	return id, f, nil
}

// Install lays out a downloaded addon package in dir using its type's factory.
func (m *Manager) Install(ctx context.Context, packageName, dir string, content []byte, downloadURL string, signature []byte) error {
	id, f, err := m.factoryFor(packageName)
	if err != nil {
		return err
	}
	return f.Install(ctx, InstallSettings{
		ID:          id,
		Dir:         dir,
		Content:     content,
		DownloadURL: downloadURL,
		Signature:   signature,
		Logger:      m.logger.With(zap.String("addon", packageName)),
	})
}

// Unload shuts down the addon with the given name and forgets it, leaving its
// files in place. It is called before the files are replaced or deleted.
func (m *Manager) Unload(ctx context.Context, packageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[packageName]
	if !ok {
		return nil
	}
	delete(m.instances, packageName)
	return inst.addon.Shutdown(ctx)
}

// Load registers the addon installed in dir. Any previously loaded addon with
// the same name is shut down first. Nothing is started until the next
// Reconcile.
func (m *Manager) Load(ctx context.Context, packageName, dir string) error {
	id, f, err := m.factoryFor(packageName)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if previous, ok := m.instances[packageName]; ok {
		delete(m.instances, packageName)
		if shutdownErr := previous.addon.Shutdown(ctx); shutdownErr != nil {
			m.logger.Warn("Could not shut down previous addon", zap.String("addon", packageName), zap.Error(shutdownErr))
		}
	}

	a, err := f.Create(ctx, Settings{
		ID:     id,
		Dir:    dir,
		Logger: m.logger.With(zap.String("addon", packageName)),
	})
	if err != nil {
		return fmt.Errorf("create addon %q: %w", packageName, err)
	}

	m.instances[packageName] = &instance{id: id, dir: dir, addon: a}
	return nil
}

// Reconcile configures and starts every loaded addon. configs maps package
// names to the configuration offered for them; an addon absent from configs is
// configured with nil. Configure is only called when an addon's configuration
// differs from what was last applied, so an unchanged offer is a no-op.
//
// Addons are independent: a failure in one does not stop the others. The
// returned Effect combines the effects of every addon.
func (m *Manager) Reconcile(ctx context.Context, configs map[string][]byte) (Effect, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name := range configs {
		if _, ok := m.instances[name]; !ok {
			// Configuration may arrive before its package is installed. It is
			// applied by the reconciliation that follows the package sync.
			m.logger.Debug("Configuration offered for an addon that is not installed", zap.String("addon", name))
		}
	}

	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	sort.Strings(names)

	var (
		effect Effect
		errs   []error
	)
	for _, name := range names {
		inst := m.instances[name]
		body := configs[name]

		if !inst.configured || !bytes.Equal(inst.lastConfig, body) {
			e, err := inst.addon.Configure(ctx, body)
			if err != nil {
				errs = append(errs, fmt.Errorf("configure addon %q: %w", name, err))
				continue
			}
			effect = effect.Merge(e)
			inst.configured = true
			inst.lastConfig = bytes.Clone(body)
		}

		e, err := inst.addon.Start(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("start addon %q: %w", name, err))
			continue
		}
		effect = effect.Merge(e)
	}

	return effect, errors.Join(errs...)
}

// Get returns the loaded addon with the given name and the directory it is
// installed in.
func (m *Manager) Get(packageName string) (Addon, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[packageName]
	if !ok {
		return nil, "", false
	}
	return inst.addon, inst.dir, true
}

// Names returns the names of the loaded addons, sorted.
func (m *Manager) Names() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.instances))
	for name := range m.instances {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Shutdown shuts down every loaded addon, leaving them loaded so that a later
// Reconcile starts them again.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, inst := range m.instances {
		if err := inst.addon.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shut down addon %q: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
