// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package fileaddon implements the "file" addon type: files the Collector reads
// from a stable path, such as a GeoIP database for the geoip processor. The
// Supervisor runs nothing for a file addon.
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
	return fileAddon{id: set.ID}, nil
}

type fileAddon struct {
	id addon.ID
}

// Configure rejects configuration: a file addon has nothing to configure, and
// silently dropping configuration a server sent would hide a server mistake.
func (a fileAddon) Configure(_ context.Context, body []byte) (addon.Effect, error) {
	if body != nil {
		return addon.Effect{}, fmt.Errorf("file addon %q does not accept configuration", a.id)
	}
	return addon.Effect{}, nil
}

func (fileAddon) Start(context.Context) (addon.Effect, error) { return addon.Effect{}, nil }

func (fileAddon) Shutdown(context.Context) error { return nil }

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
