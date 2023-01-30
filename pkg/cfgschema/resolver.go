// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/cfgschema"

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

// dirResolver is used to resolve the base directory of a given reflect.Type.
type dirResolver struct {
	srcRoot    string
	moduleName string
}

// typeToPackagePath accepts a Type and returns the filesystem path. If the
// path does not exist in the current project, returns location in GOPATH.
func (dr dirResolver) typeToPackagePath(t reflect.Type) (string, error) {
	pkgPath := t.PkgPath()
	if strings.HasPrefix(pkgPath, dr.moduleName) {
		return dr.packagePathToVerifiedProjectPath(strings.TrimPrefix(pkgPath, dr.moduleName))
	}
	verifiedGoPath, err := dr.packagePathToVerifiedGoPath(pkgPath)
	if err != nil {
		return "", fmt.Errorf("could not find the pkg %q: %w", pkgPath, err)
	}
	return verifiedGoPath, nil
}

// reflectValueToProjectPath accepts a reflect.Value and returns its directory in the current project. If
// the type doesn't live in the current project, returns "".
func (dr dirResolver) reflectValueToProjectPath(v reflect.Value) string {
	t := v.Type().Elem()
	if !strings.HasPrefix(t.PkgPath(), dr.moduleName) {
		return ""
	}
	trimmed := strings.TrimPrefix(t.PkgPath(), dr.moduleName+"/")
	dir, err := dr.packagePathToVerifiedProjectPath(trimmed)
	if err != nil {
		return ""
	}
	return dir
}

func (dr dirResolver) packagePathToVerifiedProjectPath(packagePath string) (string, error) {
	dir := dr.packagePathToProjectPath(packagePath)
	_, err := os.ReadDir(dir)
	return dir, err
}

// packagePathToProjectPath returns the path to a package in the local project.
func (dr dirResolver) packagePathToProjectPath(packagePath string) string {
	return filepath.Join(dr.srcRoot, packagePath)
}

func (dr dirResolver) packagePathToVerifiedGoPath(packagePath string) (string, error) {
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
func (dr dirResolver) packagePathToGoPath(packagePath string) (string, error) {
	gomodPath := filepath.Join(dr.srcRoot, "go.mod")
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
