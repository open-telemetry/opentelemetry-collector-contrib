// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

// ErrRuntimeNotFound reports that an addon type needs a runtime the host does
// not have, for example a JVM for the jmx type. Types return it from Install so
// that the failure is reported to the server as INSTALL_FAILED with a reason,
// rather than surfacing later as an addon that silently never runs.
var ErrRuntimeNotFound = errors.New("required runtime not found on the host")

// Factory implements one addon type. A distribution adds types by passing
// factories to the Supervisor; the Supervisor never runs an addon whose type
// has no factory.
type Factory interface {
	// Type returns the addon type this factory implements.
	Type() Type

	// Install lays out a downloaded package in a directory. It is called on a
	// fresh, empty staging directory that replaces the installed one only if
	// Install succeeds, so a failing Install leaves the previous version in
	// place. The content has already been verified against its offered hash.
	//
	// An error is reported to the server as INSTALL_FAILED, so Install is the
	// place to check prerequisites such as a runtime.
	Install(ctx context.Context, set InstallSettings) error

	// Create returns the runtime object for an addon installed in set.Dir. It
	// must not start anything.
	Create(ctx context.Context, set Settings) (Addon, error)
}

// InstallSettings are passed to Factory.Install.
type InstallSettings struct {
	ID ID
	// Dir is an empty directory to install into.
	Dir string
	// Content is the package file exactly as downloaded and verified.
	Content []byte
	// DownloadURL is where Content came from. Its path suffix is the only hint
	// OpAMP gives about the content's format.
	DownloadURL string
	// Signature is the offered signature, if any. Verifying it is the type's
	// decision.
	Signature []byte
	Logger    *zap.Logger
}

// Settings are passed to Factory.Create.
type Settings struct {
	ID ID
	// Dir is the directory the addon is installed in.
	Dir    string
	Logger *zap.Logger
}

// Addon is an installed addon.
//
// The Supervisor reconciles addons rather than driving them with events:
// after every package sync and every remote config change, and once at
// startup, it calls Configure if the addon's configuration changed and then
// Start. Both must therefore be idempotent. None of the methods is called
// concurrently with another on the same addon.
type Addon interface {
	// Configure applies configuration delivered for this addon in the remote
	// config. A nil body means no configuration is offered for it, including
	// when previously offered configuration was removed.
	Configure(ctx context.Context, body []byte) (Effect, error)

	// Start brings the addon into operation if it is not already. It is called
	// on every reconciliation, and must return a zero Effect when it changed
	// nothing.
	Start(ctx context.Context) (Effect, error)

	// Shutdown stops anything the addon runs, leaving its files in place. It is
	// called before an upgrade replaces the addon's files, before the addon is
	// deleted, and when the Supervisor stops.
	Shutdown(ctx context.Context) error
}

// Effect is what applying a change to an addon requires of the Supervisor.
type Effect struct {
	// RestartAgent asks the Supervisor to restart (or reload) the Collector,
	// for addons the Collector itself reads. The Supervisor coalesces requests
	// so that one reconciliation causes at most one restart, together with any
	// restart needed for a change to the Collector's own configuration.
	RestartAgent bool
}

// Merge combines two effects.
func (e Effect) Merge(other Effect) Effect {
	return Effect{RestartAgent: e.RestartAgent || other.RestartAgent}
}
