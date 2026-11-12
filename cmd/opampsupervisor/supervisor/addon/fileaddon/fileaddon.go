// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package fileaddon implements the "file" addon type: files the Collector reads
// from a stable path, such as a GeoIP database for the geoip processor. The
// Supervisor runs nothing for a file addon, but restarts the Collector when one
// is installed or upgraded, since components read their files at startup.
package fileaddon

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
)

// Type is the addon type implemented by this package.
const Type addon.Type = "file"

// NewFactory returns the factory for file addons.
func NewFactory() addon.Factory {
	return factory{}
}

type factory struct{}

func (factory) Type() addon.Type { return Type }

// Install extracts a gzipped tarball, or stores any other content as a single
// file named after the last segment of its download URL. The URL suffix is the
// only format hint OpAMP provides, so a raw file can be served as-is without
// wrapping it in an archive.
func (factory) Install(_ context.Context, set addon.InstallSettings) error {
	name, err := fileName(set.DownloadURL)
	if err != nil {
		return err
	}

	if isTarGz(name) {
		return addon.ExtractTarGz(set.Content, set.Dir)
	}

	if err := os.WriteFile(filepath.Join(set.Dir, name), set.Content, 0o600); err != nil {
		return fmt.Errorf("write %q: %w", name, err)
	}
	return nil
}

func (factory) Create(_ context.Context, set addon.Settings) (addon.Addon, error) {
	return &fileAddon{id: set.ID}, nil
}

type fileAddon struct {
	id addon.ID
	// started is false until the first Start. A new addon object is created for
	// each install and upgrade, so the first Start is when its files are new.
	started bool
}

// Configure rejects configuration: a file addon has nothing to configure, and
// silently dropping configuration a server sent would hide a server mistake.
func (a *fileAddon) Configure(_ context.Context, body []byte) (addon.Effect, error) {
	if body != nil {
		return addon.Effect{}, fmt.Errorf("file addon %q does not accept configuration", a.id)
	}
	return addon.Effect{}, nil
}

// Start asks for a Collector restart the first time, so that the Collector
// reads files that were just installed or upgraded. Components such as the
// geoip processor open their files at startup and would otherwise keep the old
// ones until something else restarts the Collector. At Supervisor startup the
// request is moot: addons are reconciled before the Collector first starts.
func (a *fileAddon) Start(context.Context) (addon.Effect, error) {
	if a.started {
		return addon.Effect{}, nil
	}
	a.started = true
	return addon.Effect{RestartAgent: true}, nil
}

func (*fileAddon) Shutdown(context.Context) error { return nil }

// fileName returns the file name a download URL points at.
func fileName(downloadURL string) (string, error) {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("parse download URL: %w", err)
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" || name == ".." {
		return "", errors.New("download URL does not end in a file name")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("download URL file name %q is not a plain file name", name)
	}
	return name, nil
}

func isTarGz(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz")
}
