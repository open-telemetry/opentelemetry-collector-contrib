// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// DefaultSrcRoot is the default root of the collector repo, relative to the
// current working directory. Can be used to create a DirResolver.
const DefaultSrcRoot = "."

// DefaultModule is the module prefix of contrib. Can be used to create a
// DirResolver.
const DefaultModule = "github.com/open-telemetry/opentelemetry-collector-contrib"

type DirResolverIntf interface {
	TypeToPackagePath(t reflect.Type) (string, error)
	ReflectValueToProjectPath(v reflect.Value) string
}

// DirResolver is used to resolve the base directory of a given reflect.Type.
type DirResolver struct {
	SrcRoot    string
	ModuleName string
}

// NewDefaultDirResolver creates a DirResolver with a default SrcRoot and
// ModuleName, suitable for using this package's API using otelcol with an
// executable running from the otelcol's source root (not tests).
func NewDefaultDirResolver() DirResolver {
	return NewDirResolver(DefaultSrcRoot, DefaultModule)
}

// NewDirResolver creates a DirResolver with a custom SrcRoot and ModuleName.
// Useful for testing and for using this package's API from a repository other
// than otelcol (e.g. contrib).
func NewDirResolver(srcRoot string, moduleName string) DirResolver {
	return DirResolver{
		SrcRoot:    srcRoot,
		ModuleName: moduleName,
	}
}

// TypeToPackagePath accepts a Type and returns the filesystem path. If the
// path does not exist in the current project, returns location in GOPATH.
func (dr DirResolver) TypeToPackagePath(t reflect.Type) (string, error) {
	pkgPath := t.PkgPath()
	if strings.HasPrefix(pkgPath, dr.ModuleName) {
		return dr.packagePathToVerifiedProjectPath(strings.TrimPrefix(pkgPath, dr.ModuleName+"/"))
	}
	verifiedGoPath, err := dr.packagePathToVerifiedGoPath(pkgPath)
	if err != nil {
		return "", fmt.Errorf("could not find the pkg %q: %w", pkgPath, err)
	}
	return verifiedGoPath, nil
}

// ReflectValueToProjectPath accepts a reflect.Value and returns its directory in the current project. If
// the type doesn't live in the current project, returns "".
func (dr DirResolver) ReflectValueToProjectPath(v reflect.Value) string {
	t := v.Type().Elem()
	if !strings.HasPrefix(t.PkgPath(), dr.ModuleName) {
		return ""
	}
	trimmed := strings.TrimPrefix(t.PkgPath(), dr.ModuleName+"/")
	dir, err := dr.packagePathToVerifiedProjectPath(trimmed)
	if err != nil {
		return ""
	}
	return dir
}

func (dr DirResolver) packagePathToVerifiedProjectPath(packagePath string) (string, error) {
	dir := dr.packagePathToProjectPath(packagePath)
	_, err := os.ReadDir(dir)
	return dir, err
}

// packagePathToProjectPath returns the path to a package in the local project.
func (dr DirResolver) packagePathToProjectPath(packagePath string) string {
	return filepath.Join(dr.SrcRoot, packagePath)
}

func (dr DirResolver) packagePathToVerifiedGoPath(packagePath string) (string, error) {
	dir, err := dr.packagePathToGoPath(packagePath)
	if err != nil {
		return "", err
	}
	_, err = os.ReadDir(dir)
	return dir, err
}

// packagePathToGoPath accepts a package path (e.g.
// "go.opentelemetry.io/collector/receiver/otlpreceiver") and returns the
// filesystem path starting at GOPATH by reading the current module's go.mod
// file to get the version and appending it to the filesystem path.
func (dr DirResolver) packagePathToGoPath(packagePath string) (string, error) {
	gomodPath := filepath.Join(dr.SrcRoot, "go.mod")
	gomodBytes, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", err
	}
	parsedModfile, err := modfile.Parse(gomodPath, gomodBytes, nil)
	if err != nil {
		return "", err
	}
	for _, modfileRequire := range parsedModfile.Require {
		modpath := modfileRequire.Syntax.Token[0]
		if strings.HasPrefix(packagePath, modpath) {
			return modfileRequreToGoPath(modfileRequire)
		}
	}
	return "", fmt.Errorf("packagePathToGoPath: not found in go.mod: package path %q", packagePath)
}

// modfileRequreToGoPath converts a modfile.Require value to a fully-qualified
// filesystem path with a GOPATH prefix.
func modfileRequreToGoPath(required *modfile.Require) (string, error) {
	path, err := requireTokensToPartialPath(required.Syntax.Token)
	if err != nil {
		return "", err
	}
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = build.Default.GOPATH
	}
	return filepath.Join(goPath, "pkg", "mod", path), nil
}

// requireTokensToPartialPath converts a string slice of length two e.g.
// ["github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector",
// "42"] from a modfile.Require struct to an escaped, partial filesystem path
// (without the GOPATH prefix) e.g.
// "github.com/!google!cloud!platform/opentelemetry-operations-go/exporter/collector@42"
func requireTokensToPartialPath(tokens []string) (string, error) {
	pathPart, err := module.EscapePath(tokens[0])
	if err != nil {
		return "", err
	}
	version := tokens[1]
	return fmt.Sprintf("%s@%s", pathPart, version), nil
}
